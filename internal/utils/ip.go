package utils

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/theblitlabs/gologger"
)

var (
	lastKnownIP string
	ipMutex     sync.RWMutex
	httpClient  *http.Client
)

func init() {
	httpClient = &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
			IdleConnTimeout:   1 * time.Second,
		},
	}
}

func GetPublicIP() (string, error) {
	services := []string{
		"https://api.ipify.org",
		"https://checkip.amazonaws.com",
		"https://ipv4.icanhazip.com",
	}

	resultChan := make(chan string, len(services))
	errorChan := make(chan error, len(services))

	var wg sync.WaitGroup
	for _, service := range services {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				errorChan <- err
				return
			}

			req.Header.Set("User-Agent", "parity-runner")
			req.Header.Set("Accept", "text/plain")

			resp, err := httpClient.Do(req)
			if err != nil {
				errorChan <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				errorChan <- fmt.Errorf("HTTP Error: %s returned status %s", url, http.StatusText(resp.StatusCode))
				return
			}

			ipBytes, err := io.ReadAll(io.LimitReader(resp.Body, 64))
			if err != nil {
				errorChan <- err
				return
			}

			ip := strings.TrimSpace(string(ipBytes))
			if ip != "" {
				resultChan <- ip
			} else {
				errorChan <- fmt.Errorf("empty response from %s", url)
			}
		}(service)
	}

	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	var numErrors int
	totalServices := len(services)
	var lastErr error

	for {
		select {
		case ip, ok := <-resultChan:
			if !ok {
				continue
			}
			return ip, nil
		case err, ok := <-errorChan:
			if !ok {
				continue
			}
			numErrors++
			lastErr = err

			if numErrors >= totalServices {

				ipMutex.RLock()
				if lastKnownIP != "" {
					ip := lastKnownIP
					ipMutex.RUnlock()
					return ip, nil
				}
				ipMutex.RUnlock()

				return "", fmt.Errorf("network transition: %w", lastErr)
			}
		}
	}
}

func CheckIPChanged() (string, bool, error) {
	log := gologger.WithComponent("ip_monitor")

	if !hasNetworkConnectivity() {

		ipMutex.RLock()
		if lastKnownIP != "" {
			ip := lastKnownIP
			ipMutex.RUnlock()
			return ip, false, nil
		}
		ipMutex.RUnlock()

		return "", false, fmt.Errorf("no network connectivity")
	}

	currentIP, err := GetPublicIP()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get public IP")
		return "", false, err
	}

	ipMutex.RLock()
	lastIP := lastKnownIP
	ipMutex.RUnlock()

	hasChanged := lastIP != "" && lastIP != currentIP

	ipMutex.Lock()
	lastKnownIP = currentIP
	ipMutex.Unlock()

	return currentIP, hasChanged, nil
}

func hasNetworkConnectivity() bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}

			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
					return true
				}
			}
		}
	}

	return false
}
