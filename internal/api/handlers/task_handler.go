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

	go func() {
		for {
			select {
			case <-done:
				return
			default:
				_, _, err := conn.ReadMessage()
				if err != nil {
					if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
						log.Debug().Err(err).Msg("Connection closed")
					}
					return
				}
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			tasks, err := h.service.ListAvailableTasks(context.Background())
			if err != nil {
				log.Error().Err(err).Msg("Task fetch failed")
				if err := conn.WriteJSON(WSMessage{
					Type:    "error",
					Payload: "Internal server error",
				}); err != nil {
					log.Debug().Err(err).Msg("Error message send failed")
				}
				continue
			}

			if err := conn.WriteJSON(WSMessage{
				Type:    "available_tasks",
				Payload: tasks,
			}); err != nil {
				log.Debug().Err(err).Msg("Task send failed")
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

	if deviceID == "" {
		log.Debug().Str("task", taskID).Msg("Missing device ID")
		http.Error(w, "Device ID required", http.StatusBadRequest)
		return
	}

	var result models.TaskResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		log.Debug().Err(err).
			Str("task", taskID).
			Str("device", deviceID).
			Msg("Invalid result payload")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	task, err := h.service.GetTask(r.Context(), taskID)
	if err != nil {
		log.Error().Err(err).
			Str("task", taskID).
			Str("device", deviceID).
			Msg("Task fetch failed")
		http.Error(w, "Task fetch failed", http.StatusInternalServerError)
		return
	}

	if task.CreatorID == uuid.Nil {
		log.Debug().
			Str("task", taskID).
			Str("device", deviceID).
			Msg("Missing creator ID")
		http.Error(w, "Creator ID required", http.StatusBadRequest)
		return
	}

	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		log.Debug().
			Str("task", taskID).
			Str("device", deviceID).
			Msg("Invalid task ID")
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

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

	if err := h.service.SaveTaskResult(r.Context(), &result); err != nil {
		if strings.Contains(err.Error(), "invalid task result:") {
			log.Debug().Err(err).
				Str("task", taskID).
				Str("device", deviceID).
				Msg("Invalid result")
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			log.Error().Err(err).
				Str("task", taskID).
				Str("device", deviceID).
				Msg("Result save failed")
			http.Error(w, "Result save failed", http.StatusInternalServerError)
		}
		return
	}

	if result.ExitCode == 0 {
		if err := h.distributeRewards(r.Context(), &result); err != nil {
			log.Error().Err(err).
				Str("task", taskID).
				Str("device", deviceID).
				Str("runner", result.RunnerAddress).
				Msg("Reward distribution failed")
		} else {
			log.Info().
				Str("task", taskID).
				Str("device", deviceID).
				Str("runner", result.RunnerAddress).
				Float64("reward", result.Reward).
				Msg("Task completed with rewards")
		}
	} else {
		log.Info().
			Str("task", taskID).
			Str("device", deviceID).
			Int("exit", result.ExitCode).
			Msg("Task completed with error")
	}

	w.WriteHeader(http.StatusOK)
}

func (h *TaskHandler) distributeRewards(ctx context.Context, result *models.TaskResult) error {
	log := logger.WithComponent("rewards")

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		return fmt.Errorf("config load failed: %w", err)
	}

	privateKey, err := keystore.GetPrivateKey()
	if err != nil {
		return fmt.Errorf("auth required: %w", err)
	}

	client, err := wallet.NewClientWithKey(
		cfg.Ethereum.RPC,
		big.NewInt(cfg.Ethereum.ChainID),
		privateKey,
	)
	if err != nil {
		return fmt.Errorf("wallet client failed: %w", err)
	}

	recipientAddr := client.Address()
	log.Debug().
		Str("device", result.DeviceID).
		Str("recipient", recipientAddr.Hex()).
		Msg("Using auth wallet")

	stakeWallet, err := stakewallet.NewStakeWallet(
		common.HexToAddress(cfg.Ethereum.StakeWalletAddress),
		client,
	)
	if err != nil {
		return fmt.Errorf("stake wallet init failed: %w", err)
	}

	balance, err := stakeWallet.GetBalanceByDeviceID(&bind.CallOpts{}, result.DeviceID)
	if err != nil {
		log.Error().
			Err(err).
			Str("device", result.DeviceID).
			Str("addr", cfg.Ethereum.StakeWalletAddress).
			Msg("Balance check failed")
		return fmt.Errorf("invalid device ID format")
	}

	if balance.Cmp(big.NewInt(0)) == 0 {
		log.Debug().
			Str("device", result.DeviceID).
			Msg("No stake found")
		return nil
	}

	log.Debug().
		Str("device", result.DeviceID).
		Str("balance", balance.String()).
		Msg("Found stake")

	task, err := h.service.GetTask(ctx, result.TaskID.String())
	if err != nil {
		return fmt.Errorf("task fetch failed: %w", err)
	}

	rewardWei := new(big.Int).Mul(
		big.NewInt(int64(task.Reward)),
		big.NewInt(1e18),
	)

	txOpts, err := client.GetTransactOpts()
	if err != nil {
		return fmt.Errorf("tx opts failed: %w", err)
	}

	log.Debug().
		Str("device", result.DeviceID).
		Str("recipient", recipientAddr.Hex()).
		Str("reward", rewardWei.String()).
		Msg("Initiating transfer")

	tx, err := stakeWallet.TransferPayment(
		txOpts,
		task.CreatorDeviceID,
		result.DeviceID,
		rewardWei,
	)
	if err != nil {
		log.Error().
			Err(err).
			Str("recipient", recipientAddr.Hex()).
			Str("reward", rewardWei.String()).
			Msg("Transfer failed")
		return fmt.Errorf("transfer failed: %w", err)
	}

	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return fmt.Errorf("confirmation failed: %w", err)
	}

	if receipt.Status == 0 {
		return fmt.Errorf("transfer reverted")
	}

	return nil
}

func (h *TaskHandler) checkStakeBalance(_ context.Context, task *models.Task) error {
	if h.stakeWallet == nil {
		return fmt.Errorf("stake wallet not initialized")
	}

	// Convert reward to wei (assuming reward is in whole tokens)
	rewardWei := new(big.Float).Mul(
		new(big.Float).SetFloat64(task.Reward),
		new(big.Float).SetFloat64(1e18),
	)
	rewardAmount, _ := rewardWei.Int(nil)

	// Check device stake balance
	stakeInfo, err := h.stakeWallet.GetStakeInfo(&bind.CallOpts{}, task.CreatorDeviceID)
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
