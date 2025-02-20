package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
	"github.com/theblitlabs/parity-protocol/pkg/wallet"
)

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

func (h *TaskHandler) HandleWebSocket(conn *websocket.Conn) {
	log := logger.WithComponent("websocket")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	done := make(chan struct{})
	defer close(done)

	// Start read pump in goroutine
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				_, _, err := conn.ReadMessage()
				if err != nil {
					if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
						log.Debug().Err(err).Msg("WebSocket connection closed") // Downgraded to debug since this is expected behavior
					}
					return
				}
			}
		}
	}()

	// Write pump in main goroutine
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			tasks, err := h.service.ListAvailableTasks(context.Background())
			if err != nil {
				log.Error().Err(err).Msg("Failed to fetch available tasks")
				if err := conn.WriteJSON(WSMessage{
					Type:    "error",
					Payload: "Internal server error", // Don't expose internal error details
				}); err != nil {
					log.Debug().Err(err).Msg("Failed to send error message to client") // Downgraded to debug
				}
				continue
			}

			if err := conn.WriteJSON(WSMessage{
				Type:    "available_tasks",
				Payload: tasks,
			}); err != nil {
				log.Debug().Err(err).Msg("Failed to send tasks to client") // Downgraded to debug
				return
			}
		}
	}
}

func (h *TaskHandler) GetTaskResult(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]
	if taskID == "" {
		http.Error(w, "task ID is required", http.StatusBadRequest)
		return
	}

	result, err := h.service.GetTaskResult(r.Context(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if result == nil {
		http.Error(w, "task result not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *TaskHandler) SaveTaskResult(w http.ResponseWriter, r *http.Request) {
	log := logger.WithComponent("task_handler")
	vars := mux.Vars(r)
	taskID := vars["id"]
	deviceID := r.Header.Get("X-Device-ID")

	// Validate inputs
	if deviceID == "" {
		log.Warn().Str("task_id", taskID).Msg("Missing device ID in request")
		http.Error(w, "Device ID is required", http.StatusBadRequest)
		return
	}

	var result models.TaskResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		log.Warn().Err(err).Str("task_id", taskID).Str("device_id", deviceID).Msg("Invalid task result payload")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get task details
	task, err := h.service.GetTask(r.Context(), taskID)
	if err != nil {
		log.Error().Err(err).
			Str("task_id", taskID).
			Str("device_id", deviceID).
			Msg("Failed to retrieve task details")
		http.Error(w, "Failed to get task details", http.StatusInternalServerError)
		return
	}

	if task.CreatorID == uuid.Nil {
		log.Warn().
			Str("task_id", taskID).
			Str("device_id", deviceID).
			Msg("Task has no creator ID")
		http.Error(w, "Creator device ID is required", http.StatusBadRequest)
		return
	}

	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		log.Warn().
			Str("task_id", taskID).
			Str("device_id", deviceID).
			Msg("Invalid task ID format")
		http.Error(w, "Invalid task ID format", http.StatusBadRequest)
		return
	}

	// Prepare result
	result.TaskID = taskUUID
	result.DeviceID = deviceID
	result.CreatorAddress = task.CreatorAddress
	result.CreatorDeviceID = task.CreatorDeviceID
	result.SolverDeviceID = deviceID
	result.RunnerAddress = deviceID
	result.CreatedAt = time.Now()
	result.Reward = task.Reward

	hash := sha256.Sum256([]byte(deviceID))
	result.DeviceIDHash = hex.EncodeToString(hash[:])
	result.Clean()

	// Save result
	if err := h.service.SaveTaskResult(r.Context(), &result); err != nil {
		if strings.Contains(err.Error(), "invalid task result:") {
			log.Warn().Err(err).
				Str("task_id", taskID).
				Str("device_id", deviceID).
				Msg("Invalid task result")
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			log.Error().Err(err).
				Str("task_id", taskID).
				Str("device_id", deviceID).
				Msg("Failed to save task result")
			http.Error(w, "Failed to save task result", http.StatusInternalServerError)
		}
		return
	}

	// Handle rewards for successful tasks
	if result.ExitCode == 0 {
		if err := h.distributeRewards(r.Context(), &result); err != nil {
			log.Error().Err(err).
				Str("task_id", taskID).
				Str("device_id", deviceID).
				Str("runner", result.RunnerAddress).
				Msg("Failed to distribute rewards")
		} else {
			log.Info().
				Str("task_id", taskID).
				Str("device_id", deviceID).
				Str("runner", result.RunnerAddress).
				Float64("reward", result.Reward).
				Msg("Task completed and rewards distributed")
		}
	} else {
		log.Info().
			Str("task_id", taskID).
			Str("device_id", deviceID).
			Int("exit_code", result.ExitCode).
			Msg("Task completed with non-zero exit code")
	}

	w.WriteHeader(http.StatusOK)
}

func (h *TaskHandler) distributeRewards(ctx context.Context, result *models.TaskResult) error {
	log := logger.WithComponent("rewards")

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get private key from keystore
	privateKey, err := keystore.GetPrivateKey()
	if err != nil {
		return fmt.Errorf("failed to get private key from keystore: %w", err)
	}

	// Create client with keystore private key
	client, err := wallet.NewClientWithKey(
		cfg.Ethereum.RPC,
		big.NewInt(cfg.Ethereum.ChainID),
		privateKey,
	)
	if err != nil {
		return fmt.Errorf("failed to create wallet client: %w", err)
	}

	recipientAddr := client.Address()
	log.Debug(). // Downgraded to debug since this is internal detail
			Str("device_id", result.DeviceID).
			Str("recipient", recipientAddr.Hex()).
			Msg("Using authenticated wallet for rewards")

	stakeWallet, err := stakewallet.NewStakeWallet(
		common.HexToAddress(cfg.Ethereum.StakeWalletAddress),
		client,
	)
	if err != nil {
		return fmt.Errorf("failed to create stake wallet contract: %w", err)
	}

	balance, err := stakeWallet.GetBalanceByDeviceID(&bind.CallOpts{}, result.DeviceID)
	if err != nil {
		log.Error().
			Err(err).
			Str("device_id", result.DeviceID).
			Str("stake_wallet", cfg.Ethereum.StakeWalletAddress).
			Msg("Failed to verify stake balance")
		return fmt.Errorf("contract call reverted - please verify device ID format")
	}

	if balance.Cmp(big.NewInt(0)) == 0 {
		log.Warn().
			Str("device_id", result.DeviceID).
			Msg("No stake found for device - please stake tokens using 'parity stake'")
		return nil
	}

	log.Info().
		Str("device_id", result.DeviceID).
		Str("balance", balance.String()).
		Msg("Found staked tokens for device")

	// Get the actual task to access creator information
	task, err := h.service.GetTask(ctx, result.TaskID.String())
	if err != nil {
		return fmt.Errorf("failed to get task details: %w", err)
	}

	// Convert task reward to wei
	rewardWei := new(big.Int).Mul(
		big.NewInt(int64(task.Reward)),
		big.NewInt(1e18),
	)

	// Get transaction options from the authenticated client
	txOpts, err := client.GetTransactOpts()
	if err != nil {
		return fmt.Errorf("failed to get transaction options: %w", err)
	}

	log.Info().
		Str("device_id", result.DeviceID).
		Str("recipient", recipientAddr.Hex()).
		Str("reward_amount", rewardWei.String()).
		Msg("Distributing rewards to authenticated wallet")

	// Transfer payment from creator to solver
	tx, err := stakeWallet.TransferPayment(
		txOpts,
		task.CreatorDeviceID, // Use creator's device ID from task
		result.DeviceID,      // Runner's device ID
		rewardWei,
	)
	if err != nil {
		log.Error().
			Err(err).
			Str("recipient", recipientAddr.Hex()).
			Str("reward", rewardWei.String()).
			Msg("TransferPayment failed")
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

func (h *TaskHandler) checkStakeBalance(_ context.Context, task *models.Task) error {
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client, err := wallet.NewClient(cfg.Ethereum.RPC, cfg.Ethereum.ChainID)
	if err != nil {
		return fmt.Errorf("failed to create wallet client: %w", err)
	}

	stakeWallet, err := stakewallet.NewStakeWallet(
		common.HexToAddress(cfg.Ethereum.StakeWalletAddress),
		client,
	)
	if err != nil {
		return fmt.Errorf("failed to create stake wallet contract: %w", err)
	}

	// Convert reward to wei (assuming reward is in whole tokens)
	rewardWei := new(big.Float).Mul(
		new(big.Float).SetFloat64(task.Reward),
		new(big.Float).SetFloat64(1e18),
	)
	rewardAmount, _ := rewardWei.Int(nil)

	// Check device stake balance
	stakeInfo, err := stakeWallet.GetStakeInfo(&bind.CallOpts{}, task.CreatorDeviceID)
	if err != nil || !stakeInfo.Exists {
		return fmt.Errorf("creator device not registered - please stake first")
	}

	if stakeInfo.Amount.Cmp(rewardAmount) < 0 {
		return fmt.Errorf("insufficient stake balance: need %v PRTY, has %v PRTY",
			task.Reward,
			new(big.Float).Quo(new(big.Float).SetInt(stakeInfo.Amount), big.NewFloat(1e18)))
	}

	return nil
}
