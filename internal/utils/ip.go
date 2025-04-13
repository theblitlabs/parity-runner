package utils
import (
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"
    
    "github.com/theblitlabs/gologger"
)

var (
    lastKnownIP string
)

func GetPublicIP() (string, error) {
    client := &http.Client{
        Timeout: 5 * time.Second,
    }

    services := []string{
        "https://api.ipify.org",
        "https://checkip.amazonaws.com",
        "https://ipv4.icanhazip.com",
    }

    var lastErr error
    for _, service := range services {
        resp, err := client.Get(service)
        if (err != nil) {
            lastErr = err
            continue
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            lastErr = fmt.Errorf("HTTP Error: %s returned status %s", service, http.StatusText(resp.StatusCode))
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

func CheckIPChanged() (string, bool, error) {
    log := gologger.WithComponent("ip_monitor")
    
    log.Debug().Msg("Checking for IP changes...")
    
    currentIP, err := GetPublicIP()
    if err != nil {
        log.Error().Err(err).Msg("Failed to get public IP")
        return "", false, err
    }
    
    log.Debug().
        Str("current_ip", currentIP).
        Str("last_known_ip", lastKnownIP).
        Msg("Checking IP change status")
    
    hasChanged := lastKnownIP != "" && lastKnownIP != currentIP
    
    if hasChanged {
        log.Info().
            Str("old_ip", lastKnownIP).
            Str("new_ip", currentIP).
            Msg("Public IP changed")
    } else if lastKnownIP == "" {
        log.Info().Str("ip", currentIP).Msg("Initial IP detected")
    }
    
    // Update the last known IP
    lastKnownIP = currentIP
    
    return currentIP, hasChanged, nil
}