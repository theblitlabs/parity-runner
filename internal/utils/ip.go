package utils

import (
	"fmt"
	"io"
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

	select {
	case ip := <-resultChan:
		return ip, nil
	case err := <-errorChan:
		select {
		case ip := <-resultChan:
			return ip, nil
		default:
			return "", err
		}
	}
}

func CheckIPChanged() (string, bool, error) {
	log := gologger.WithComponent("ip_monitor")

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
