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

	if task.CreatorID == "" {
		http.Error(w, "Creator device ID is required", http.StatusBadRequest)
		return
	}

	if err := h.checkStakeBalance(r.Context(), task); err != nil {
		http.Error(w, "Task creation failed: "+err.Error(), http.StatusBadRequest)
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

	// Get the actual task to access creator information
	task, err := h.service.GetTask(ctx, result.TaskID)
	if err != nil {
		return fmt.Errorf("failed to get task details: %w", err)
	}

	// Convert task reward to wei
	rewardWei := new(big.Int).Mul(
		big.NewInt(int64(task.Reward)),
		big.NewInt(1e18),
	)

	// Get transaction options and distribute
	ownerClient, err := wallet.NewClientWithKey(
		cfg.Ethereum.RPC,
		big.NewInt(cfg.Ethereum.ChainID),
		cfg.Ethereum.OwnerPrivateKey,
	)
	if err != nil {
		return fmt.Errorf("failed to create owner client: %w", err)
	}

	txOpts, err := ownerClient.GetTransactOpts()
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
		task.CreatorID,  // Correct: Use creator's device ID
		result.DeviceID, // Runner's device ID
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
	stakeInfo, err := stakeWallet.GetStakeInfo(&bind.CallOpts{}, task.CreatorID)
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
