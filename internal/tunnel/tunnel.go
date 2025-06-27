package tunnel

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/theblitlabs/gologger"
)

type TunnelType string

const (
	TunnelTypeBore   TunnelType = "bore"
	TunnelTypeNgrok  TunnelType = "ngrok"
	TunnelTypeLocal  TunnelType = "local"
	TunnelTypeCustom TunnelType = "custom"
)

type TunnelConfig struct {
	Type      TunnelType `mapstructure:"type"`
	ServerURL string     `mapstructure:"server_url"`
	Port      int        `mapstructure:"port"`
	Secret    string     `mapstructure:"secret"`
	LocalPort int        `mapstructure:"local_port"`
	Enabled   bool       `mapstructure:"enabled"`
}

type TunnelClient struct {
	config    TunnelConfig
	cmd       *exec.Cmd
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	running   bool
	publicURL string
}

func NewTunnelClient(config TunnelConfig) *TunnelClient {
	return &TunnelClient{
		config: config,
	}
}

func (t *TunnelClient) Start() (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.running {
		return t.publicURL, nil
	}

	log := gologger.WithComponent("tunnel")

	if !t.config.Enabled {
		localURL := fmt.Sprintf("http://localhost:%d", t.config.LocalPort)
		log.Info().Str("url", localURL).Msg("Tunnel disabled, using local URL")
		return localURL, nil
	}

	// Auto-install bore if not found
	if err := t.ensureBoreInstalled(); err != nil {
		return "", fmt.Errorf("failed to ensure bore is installed: %w", err)
	}

	switch t.config.Type {
	case TunnelTypeBore:
		return t.startBoreTunnel()
	case TunnelTypeLocal:
		localURL := fmt.Sprintf("http://localhost:%d", t.config.LocalPort)
		log.Info().Str("url", localURL).Msg("Using local URL (no tunnel)")
		return localURL, nil
	default:
		return "", fmt.Errorf("unsupported tunnel type: %s", t.config.Type)
	}
}

func (t *TunnelClient) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return nil
	}

	log := gologger.WithComponent("tunnel")
	log.Info().Msg("Stopping tunnel")

	if t.cancel != nil {
		t.cancel()
	}

	if t.cmd != nil && t.cmd.Process != nil {
		if err := t.cmd.Process.Kill(); err != nil {
			log.Error().Err(err).Msg("Failed to kill tunnel process")
			return err
		}
		t.cmd.Wait()
	}

	t.running = false
	t.publicURL = ""
	log.Info().Msg("Tunnel stopped")
	return nil
}

func (t *TunnelClient) GetPublicURL() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.publicURL
}

func (t *TunnelClient) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}

func (t *TunnelClient) ensureBoreInstalled() error {
	log := gologger.WithComponent("tunnel")

	// Check if bore is already installed
	if _, err := exec.LookPath("bore"); err == nil {
		log.Debug().Msg("bore CLI already installed")
		return nil
	}

	log.Info().Msg("bore CLI not found, attempting to install...")

	switch runtime.GOOS {
	case "darwin":
		return t.installBoreMacOS()
	case "linux":
		return t.installBoreLinux()
	default:
		return fmt.Errorf("automatic bore installation not supported on %s, please install manually", runtime.GOOS)
	}
}

func (t *TunnelClient) installBoreMacOS() error {
	log := gologger.WithComponent("tunnel")

	// Try Homebrew first
	if _, err := exec.LookPath("brew"); err == nil {
		log.Info().Msg("Installing bore via Homebrew...")
		cmd := exec.Command("brew", "install", "bore-cli")
		if err := cmd.Run(); err != nil {
			log.Warn().Err(err).Msg("Homebrew installation failed, trying Cargo...")
			return t.installBoreCargo()
		}
		log.Info().Msg("bore installed successfully via Homebrew")
		return nil
	}

	return t.installBoreCargo()
}

func (t *TunnelClient) installBoreLinux() error {
	// Try Cargo first on Linux
	return t.installBoreCargo()
}

func (t *TunnelClient) installBoreCargo() error {
	log := gologger.WithComponent("tunnel")

	// Check if cargo is available
	if _, err := exec.LookPath("cargo"); err != nil {
		return fmt.Errorf("neither brew nor cargo found, please install bore manually: https://github.com/ekzhang/bore")
	}

	log.Info().Msg("Installing bore via Cargo (this may take a few minutes)...")
	cmd := exec.Command("cargo", "install", "bore-cli")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install bore via cargo: %w", err)
	}

	log.Info().Msg("bore installed successfully via Cargo")
	return nil
}

func (t *TunnelClient) startBoreTunnel() (string, error) {
	log := gologger.WithComponent("tunnel")

	serverURL := t.config.ServerURL
	if serverURL == "" {
		serverURL = "bore.pub"
	}

	args := []string{"local", fmt.Sprintf("%d", t.config.LocalPort), "--to", serverURL}

	if t.config.Port > 0 {
		args = append(args, "--port", fmt.Sprintf("%d", t.config.Port))
	}

	if t.config.Secret != "" {
		args = append(args, "--secret", t.config.Secret)
	}

	t.ctx, t.cancel = context.WithCancel(context.Background())
	t.cmd = exec.CommandContext(t.ctx, "bore", args...)

	log.Info().
		Str("server", serverURL).
		Int("local_port", t.config.LocalPort).
		Str("command", fmt.Sprintf("bore %s", strings.Join(args, " "))).
		Msg("Starting bore tunnel")

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := t.cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start bore tunnel: %w", err)
	}

	publicURL := make(chan string, 1)
	errorCh := make(chan error, 1)

	// Monitor stdout for tunnel URL with multiple regex patterns
	go func() {
		scanner := bufio.NewScanner(stdout)
		// Enhanced regex patterns to catch different bore output formats
		patterns := []*regexp.Regexp{
			regexp.MustCompile(`listening at ([^:\s]+):(\d+)`),
			regexp.MustCompile(`server listening.*?([^:\s]+):(\d+)`),
			regexp.MustCompile(`tunnel.*?([^:\s]+):(\d+)`),
			regexp.MustCompile(`connected to server.*?remote_port=(\d+)`), // Newer bore format
			regexp.MustCompile(`INFO.*?connected.*?remote_port=(\d+)`),    // With log prefix
			regexp.MustCompile(`bore-cli.*?listening.*?([^:\s]+):(\d+)`),  // Full format
		}

		for scanner.Scan() {
			line := scanner.Text()
			log.Info().Str("bore_output", line).Msg("Bore stdout")

			// Try each pattern
			for i, pattern := range patterns {
				if matches := pattern.FindStringSubmatch(line); len(matches) >= 2 {
					var url string

					// Handle different match formats
					if i == 3 || i == 4 { // remote_port patterns
						if len(matches) >= 2 {
							url = fmt.Sprintf("http://%s:%s", serverURL, matches[1])
						}
					} else if len(matches) >= 3 { // host:port patterns
						url = fmt.Sprintf("http://%s:%s", matches[1], matches[2])
					}

					if url != "" {
						log.Info().
							Str("detected_url", url).
							Str("pattern_used", fmt.Sprintf("pattern_%d", i)).
							Msg("Tunnel URL detected")
						select {
						case publicURL <- url:
						default:
						}
						return
					}
				}
			}

			// Also check for just port numbers in any line containing relevant keywords
			if strings.Contains(strings.ToLower(line), "listening") ||
				strings.Contains(strings.ToLower(line), "remote_port") ||
				strings.Contains(strings.ToLower(line), "connected") {
				// More specific regex to extract port from remote_port=XXXX
				remotePortRegex := regexp.MustCompile(`remote_port.*?=.*?(\d{4,5})`)
				if matches := remotePortRegex.FindStringSubmatch(line); len(matches) > 1 {
					url := fmt.Sprintf("http://%s:%s", serverURL, matches[1])
					log.Info().
						Str("detected_url", url).
						Str("from_line", line).
						Str("extracted_port", matches[1]).
						Msg("Tunnel URL detected from remote_port")
					select {
					case publicURL <- url:
					default:
					}
					return
				}

				// Fallback: general port extraction (but only if no remote_port found)
				if !strings.Contains(strings.ToLower(line), "remote_port") {
					portRegex := regexp.MustCompile(`(\d{4,5})`)
					if matches := portRegex.FindStringSubmatch(line); len(matches) > 1 {
						url := fmt.Sprintf("http://%s:%s", serverURL, matches[1])
						log.Info().
							Str("detected_url", url).
							Str("from_line", line).
							Str("extracted_port", matches[1]).
							Msg("Tunnel URL detected from fallback port extraction")
						select {
						case publicURL <- url:
						default:
						}
						return
					}
				}
			}
		}

		// If we get here, no URL was detected
		log.Warn().Msg("Bore process ended without detecting tunnel URL")
		errorCh <- fmt.Errorf("bore process ended without detecting tunnel URL")
	}()

	// Monitor stderr for errors
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			log.Warn().Str("bore_stderr", line).Msg("Bore stderr")

			if strings.Contains(strings.ToLower(line), "error") ||
				strings.Contains(strings.ToLower(line), "failed") ||
				strings.Contains(strings.ToLower(line), "refused") {
				select {
				case errorCh <- fmt.Errorf("bore error: %s", line):
				default:
				}
			}
		}
	}()

	// Monitor process exit
	go func() {
		if err := t.cmd.Wait(); err != nil {
			log.Error().Err(err).Msg("Bore process exited with error")
			select {
			case errorCh <- fmt.Errorf("bore process failed: %w", err):
			default:
			}
		}
	}()

	// Wait for URL or timeout
	select {
	case url := <-publicURL:
		t.publicURL = url + "/webhook"
		t.running = true
		log.Info().
			Str("public_url", t.publicURL).
			Str("base_url", url).
			Msg("Bore tunnel established successfully")
		return t.publicURL, nil
	case err := <-errorCh:
		t.Stop()
		return "", fmt.Errorf("tunnel failed: %w", err)
	case <-time.After(60 * time.Second):
		t.Stop()
		return "", fmt.Errorf("timeout waiting for bore tunnel to establish (60s)")
	case <-t.ctx.Done():
		return "", fmt.Errorf("tunnel startup cancelled")
	}
}
