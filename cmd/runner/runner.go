package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/docker/docker/client"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"github.com/virajbhartiya/parity-protocol/internal/config"
	"github.com/virajbhartiya/parity-protocol/internal/execution/sandbox"
	"github.com/virajbhartiya/parity-protocol/internal/models"
	"github.com/virajbhartiya/parity-protocol/pkg/device"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
	"github.com/virajbhartiya/parity-protocol/pkg/stakewallet"
	"github.com/virajbhartiya/parity-protocol/pkg/wallet"
)

type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func checkDockerAvailability() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	// Check if Docker daemon is running
	if _, err := cli.Ping(ctx); err != nil {
		return fmt.Errorf("docker daemon is not running: %w", err)
	}

	// Get Docker version info
	version, err := cli.ServerVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Docker version: %w", err)
	}

	log := logger.Get()
	log.Info().
		Str("version", version.Version).
		Str("api_version", version.APIVersion).
		Str("os", version.Os).
		Str("arch", version.Arch).
		Msg("Docker daemon is running")

	return nil
}

func Run() {
	log := logger.Get()
	log.Info().Msg("Starting runner")

	if err := checkDockerAvailability(); err != nil {
		log.Fatal().Err(err).Msg("Docker is not available")
	}

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// Create WebSocket URL
	wsURL := fmt.Sprintf("ws://%s:%s%s/runners/ws",
		cfg.Server.Host,
		cfg.Server.Port,
		cfg.Server.Endpoint,
	)

	// Connect to WebSocket
	log.Info().Str("url", wsURL).Msg("Connecting to WebSocket")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to WebSocket")
	}
	defer conn.Close()

	log.Info().Msg("Connected to WebSocket")

	// Create Docker executor
	executor, err := sandbox.NewDockerExecutor(&sandbox.ExecutorConfig{
		MemoryLimit: cfg.Runner.Docker.MemoryLimit,
		CPULimit:    cfg.Runner.Docker.CPULimit,
		Timeout:     cfg.Runner.Docker.Timeout,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Docker executor")
	}

	// Handle incoming messages
	for {
		var msg WSMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Error().Err(err).Msg("WebSocket read error")
			}
			return
		}

		switch msg.Type {
		case "available_tasks":
			var tasks []*models.Task
			data, _ := json.Marshal(msg.Payload)
			if err := json.Unmarshal(data, &tasks); err != nil {
				log.Error().Err(err).Msg("Failed to parse tasks")
				continue
			}

			// Process tasks
			for _, task := range tasks {
				log.Info().
					Str("task_id", task.ID).
					Str("creator_id", task.CreatorID).
					Str("status", string(task.Status)).
					Msg("Processing task")

				// Try to start task
				if err := StartTask(cfg.Runner.ServerURL, task.ID); err != nil {
					log.Error().
						Err(err).
						Str("task_id", task.ID).
						Str("url", fmt.Sprintf("%s/runners/tasks/%s/start", cfg.Runner.ServerURL, task.ID)).
						Msg("Failed to start task")
					continue
				}
				log.Info().Str("task_id", task.ID).Msg("Successfully started task")

				// Execute task
				log.Info().
					Str("task_id", task.ID).
					Msg("Beginning task execution")

				result, err := executor.ExecuteTask(context.Background(), task)
				if err != nil {
					log.Error().
						Err(err).
						Str("task_id", task.ID).
						Msg("Failed to execute task")
					continue
				}

				// Save the task result
				if err := SaveTaskResult(cfg.Runner.ServerURL, task.ID, result); err != nil {
					log.Error().
						Err(err).
						Str("task_id", task.ID).
						Msg("Failed to save task result")
					continue
				}

				log.Info().Str("task_id", task.ID).Msg("Task execution completed")

				// Mark task as completed
				if err := CompleteTask(cfg.Runner.ServerURL, task.ID); err != nil {
					log.Error().
						Err(err).
						Str("task_id", task.ID).
						Str("url", fmt.Sprintf("%s/runners/tasks/%s/complete", cfg.Runner.ServerURL, task.ID)).
						Msg("Failed to complete task")
					continue
				}
				log.Info().Str("task_id", task.ID).Msg("Successfully marked task as completed")

				// Distribute rewards
				if err := distributeRewards(result); err != nil {
					log.Error().Err(err).Msg("Failed to distribute rewards")
				}
			}
		}
	}
}

func distributeRewards(result *models.TaskResult) error {
	ctx := context.Background()
	log.Info().Msg("Checking stake status before distributing rewards")

	// Load config and create clients
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client, err := wallet.NewClientWithKey(
		cfg.Ethereum.RPC,
		big.NewInt(cfg.Ethereum.ChainID),
		cfg.Ethereum.RunnerPrivateKey,
	)
	if err != nil {
		return fmt.Errorf("failed to create ethereum client: %w", err)
	}

	stakeWalletAddr := common.HexToAddress(cfg.Ethereum.StakeWalletAddress)
	stakeWallet, err := stakewallet.NewStakeWallet(stakeWalletAddr, client)
	if err != nil {
		return fmt.Errorf("failed to create stake wallet: %w", err)
	}

	// Check if runner has staked
	stakeInfo, err := stakeWallet.GetStakeInfo(&bind.CallOpts{}, result.DeviceID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check stake info - skipping rewards")
		return nil // Don't fail the task
	}

	if !stakeInfo.Exists {
		log.Error().
			Str("runner", result.DeviceID).
			Msg("Runner has not staked - please stake tokens first using 'parity stake'")
		return nil // Don't fail the task
	}

	log.Info().
		Str("runner", result.DeviceID).
		Str("stake_amount", stakeInfo.Amount.String()).
		Msg("Found stake - distributing rewards")

	// Get transaction options
	txOpts, err := client.GetTransactOpts()
	if err != nil {
		return fmt.Errorf("failed to get transaction options: %w", err)
	}

	// Distribute rewards (assuming reward amount is configured or calculated)
	rewardAmount := big.NewInt(1e18) // 1 token as reward, adjust as needed
	tx, err := stakeWallet.TransferPayment(
		txOpts,
		result.CreatorAddress, // Queuer's device ID from task result
		result.DeviceID,       // Runner's device ID
		rewardAmount,
	)
	if err != nil {
		return fmt.Errorf("failed to distribute rewards: %w", err)
	}

	// Wait for transaction confirmation
	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return fmt.Errorf("failed to confirm reward distribution: %w", err)
	}

	if receipt.Status == 0 {
		return fmt.Errorf("reward distribution transaction failed")
	}

	return nil
}

func GetAvailableTasks(baseURL string) ([]*models.Task, error) {
	url := fmt.Sprintf("%s/runners/tasks/available", baseURL)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET failed for %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var tasks []*models.Task
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return tasks, nil
}

func StartTask(baseURL, taskID string) error {
	url := fmt.Sprintf("%s/runners/tasks/%s/start", baseURL, taskID)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add runner ID header
	req.Header.Set("X-Runner-ID", uuid.New().String())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP POST failed for %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func CompleteTask(baseURL, taskID string) error {
	url := fmt.Sprintf("%s/runners/tasks/%s/complete", baseURL, taskID)
	log := logger.Get()

	log.Debug().
		Str("url", url).
		Str("task_id", taskID).
		Msg("Marking task as completed")

	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("HTTP POST failed for %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func SaveTaskResult(baseURL, taskID string, result *models.TaskResult) error {
	url := fmt.Sprintf("%s/runners/tasks/%s/result", baseURL, taskID)

	// Get device ID
	deviceID, err := device.VerifyDeviceID()
	if err != nil {
		return fmt.Errorf("failed to get device ID: %w", err)
	}

	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add device ID header
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-ID", deviceID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP POST failed for %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
