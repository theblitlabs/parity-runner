package utils

import (
    "io"
    "net/http"
    "strings"
    "sync"
    "time"

    "github.com/theblitlabs/gologger"
)

// IPMonitor detects public IP changes and triggers callbacks when changes occur
type IPMonitor struct {
    currentIP      string
    mu             sync.RWMutex
    checkInterval  time.Duration
    stopCh         chan struct{}
    changeCallback func()
}

// NewIPMonitor creates and configures a new IP monitor instance
func NewIPMonitor(checkInterval time.Duration, changeCallback func()) *IPMonitor {
    return &IPMonitor{
        checkInterval:  checkInterval,
        stopCh:         make(chan struct{}),
        changeCallback: changeCallback,
    }
}

// Start launches the IP monitoring goroutine
func (m *IPMonitor) Start() {
    log := gologger.WithComponent("ip_monitor")
    log.Info().Msg("Starting IP monitor")

    // Init with current IP
    initialIP, err := m.getPublicIP()
    if err != nil {
        log.Error().Err(err).Msg("Failed to get initial public IP")
    } else {
        m.mu.Lock()
        m.currentIP = initialIP
        m.mu.Unlock()
        log.Info().Str("ip", initialIP).Msg("Initial public IP detected")
    }

    go func() {
        ticker := time.NewTicker(m.checkInterval)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                newIP, err := m.getPublicIP()
                if err != nil {
                    log.Error().Err(err).Msg("Failed to get public IP")
                    continue
                }

                m.mu.RLock()
                oldIP := m.currentIP
                m.mu.RUnlock()

                if newIP != oldIP && newIP != "" {
                    log.Info().
                        Str("old_ip", oldIP).
                        Str("new_ip", newIP).
                        Msg("Public IP changed")

                    m.mu.Lock()
                    m.currentIP = newIP
                    m.mu.Unlock()

                    if m.changeCallback != nil {
                        m.changeCallback()
                    }
                }

            case <-m.stopCh:
                log.Info().Msg("IP monitor stopped")
                return
            }
        }
    }()
}

// Stop shuts down the IP monitor cleanly
func (m *IPMonitor) Stop() {
    close(m.stopCh)
}

// GetCurrentIP returns the most recently detected public IP
func (m *IPMonitor) GetCurrentIP() string {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.currentIP
}

// getPublicIP queries multiple IP services with fallbacks
func (m *IPMonitor) getPublicIP() (string, error) {
    client := &http.Client{
        Timeout: 5 * time.Second,
    }

    // Try services in order until one succeeds
    services := []string{
        "https://api.ipify.org",
        "https://ifconfig.me/ip",
        "https://icanhazip.com",
    }

    var lastErr error
    for _, service := range services {
        resp, err := client.Get(service)
        if err != nil {
            lastErr = err
            continue
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            lastErr = &httpError{statusCode: resp.StatusCode, url: service}
            continue
        }

        ipBytes, err := io.ReadAll(resp.Body)
        if err != nil {
            lastErr = err
            continue
        }

        ip := strings.TrimSpace(string(ipBytes))
        if ip != "" {
            return ip, nil
        }
    }

    return "", lastErr
}

type httpError struct {
    statusCode int
    url        string
}

func (e *httpError) Error() string {
    return "HTTP Error: " + e.url + " returned status " + http.StatusText(e.statusCode)
}