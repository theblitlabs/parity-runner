package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/virajbhartiya/parity-protocol/internal/config"
	"github.com/virajbhartiya/parity-protocol/internal/models"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
	"github.com/virajbhartiya/parity-protocol/pkg/stakewallet"
	"github.com/virajbhartiya/parity-protocol/pkg/wallet"
)

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

func (h *TaskHandler) HandleWebSocket(conn *websocket.Conn) {
	log := logger.Get()
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
						log.Error().Err(err).Msg("WebSocket read error")
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
				log.Error().Err(err).Msg("Failed to get available tasks")
				// Send error message to client
				if err := conn.WriteJSON(WSMessage{
					Type:    "error",
					Payload: err.Error(),
				}); err != nil {
					log.Error().Err(err).Msg("Failed to write error message")
				}
				continue
			}

			if err := conn.WriteJSON(WSMessage{
				Type:    "available_tasks",
				Payload: tasks,
			}); err != nil {
				log.Error().Err(err).Msg("Failed to write message")
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
	log := logger.Get()
	vars := mux.Vars(r)
	taskID := vars["id"]

	// Get device ID from header
	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		log.Error().Msg("Device ID not provided")
		http.Error(w, "Device ID is required", http.StatusBadRequest)
		return
	}

	var result models.TaskResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		log.Error().Err(err).Msg("Failed to decode task result")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get task details including creator's wallet address
	task, err := h.service.GetTask(r.Context(), taskID)
	if err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to get task details")
		http.Error(w, "Failed to get task details", http.StatusInternalServerError)
		return
	}

	result.TaskID = taskID
	result.DeviceID = deviceID
	result.CreatorAddress = task.CreatorAddress // Add creator's address to result
	result.CreatedAt = time.Now()
	result.Clean()

	// Save the result first
	if err := h.service.SaveTaskResult(r.Context(), &result); err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to save task result")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If task completed successfully, try to distribute rewards
	if result.ExitCode == 0 {
		if err := h.distributeRewards(r.Context(), &result); err != nil {
			// Log the error but don't fail the request
			log.Error().Err(err).
				Str("task_id", taskID).
				Str("runner", result.RunnerAddress).
				Msg("Failed to distribute rewards - task saved successfully")
		} else {
			log.Info().
				Str("task_id", taskID).
				Str("runner", result.RunnerAddress).
				Str("device_id", deviceID).
				Msg("Rewards distributed successfully")
		}
	}

	log.Info().
		Str("task_id", taskID).
		Str("device_id", deviceID).
		Msg("Task result saved successfully")
	w.WriteHeader(http.StatusOK)
}

func (h *TaskHandler) distributeRewards(ctx context.Context, result *models.TaskResult) error {
	log := logger.Get()

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create client with authenticated wallet
	client, err := wallet.NewClient(cfg.Ethereum.RPC, cfg.Ethereum.ChainID)
	if err != nil {
		return fmt.Errorf("failed to create wallet client: %w", err)
	}

	// Use the authenticated wallet address as recipient
	recipientAddr := client.Address()
	log.Info().
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

	// Get balance using the raw device ID
	balance, err := stakeWallet.GetBalanceByDeviceID(&bind.CallOpts{}, result.DeviceID)
	if err != nil {
		log.Error().
			Err(err).
			Str("device_id", result.DeviceID).
			Str("stake_wallet", cfg.Ethereum.StakeWalletAddress).
			Msg("Failed to get device stake balance")
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

	rewardAmount := big.NewInt(1e18) // 1 token as reward
	ownerAmount := big.NewInt(0)     // No tokens to owner in this case

	// Get transaction options and distribute
	txOpts, err := client.GetTransactOpts()
	if err != nil {
		return fmt.Errorf("failed to get transaction options: %w", err)
	}

	log.Info().
		Str("device_id", result.DeviceID).
		Str("recipient", recipientAddr.Hex()).
		Str("reward_amount", rewardAmount.String()).
		Msg("Distributing rewards to authenticated wallet")

	tx, err := stakeWallet.DistributeStake(
		txOpts,
		result.DeviceID,
		recipientAddr,
		ownerAmount,
		rewardAmount,
	)
	if err != nil {
		log.Error().
			Err(err).
			Str("recipient", recipientAddr.Hex()).
			Str("reward", rewardAmount.String()).
			Msg("DistributeStake failed")
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

func (h *TaskHandler) checkStakeBalance(ctx context.Context, task *models.Task) error {
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
	balance, err := stakeWallet.GetBalanceByDeviceID(&bind.CallOpts{}, task.CreatorID)
	if err != nil {
		return fmt.Errorf("failed to check stake balance: %w", err)
	}

	if balance.Cmp(rewardAmount) < 0 {
		return fmt.Errorf("insufficient stake balance for reward amount - required: %v PRTY, available: %v PRTY",
			task.Reward,
			new(big.Float).Quo(new(big.Float).SetInt(balance), new(big.Float).SetFloat64(1e18)))
	}

	return nil
}
