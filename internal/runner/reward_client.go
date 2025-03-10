package runner

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
	"github.com/theblitlabs/parity-protocol/pkg/wallet"
)

type RewardClient interface {
	DistributeRewards(result *models.TaskResult) error
}

type StakeWallet interface {
	GetStakeInfo(opts *bind.CallOpts, deviceID string) (stakewallet.StakeInfo, error)
	TransferPayment(opts *bind.TransactOpts, creator string, runner string, amount *big.Int) error
}

type EthereumRewardClient struct {
	cfg         *config.Config
	stakeWallet StakeWallet
}

func NewEthereumRewardClient(cfg *config.Config) *EthereumRewardClient {
	return &EthereumRewardClient{
		cfg: cfg,
	}
}

// SetStakeWallet sets the stake wallet for testing
func (c *EthereumRewardClient) SetStakeWallet(sw StakeWallet) {
	c.stakeWallet = sw
}

func (c *EthereumRewardClient) DistributeRewards(result *models.TaskResult) error {
	log := logger.WithComponent("rewards").With().
		Str("task", result.TaskID.String()).
		Str("device", result.DeviceID).
		Float64("reward", result.Reward).
		Logger()

	log.Info().Msg("Starting reward distribution")

	// Validate reward amount
	if result.Reward <= 0 {
		log.Error().Msg("Invalid reward amount - must be greater than zero")
		return fmt.Errorf("invalid reward amount: must be greater than zero")
	}

	// If we have a mock stake wallet (for testing), use it
	if c.stakeWallet != nil {
		stakeInfo, err := c.stakeWallet.GetStakeInfo(&bind.CallOpts{}, result.DeviceID)
		if err != nil {
			log.Error().Err(err).Msg("Stake info check failed")
			return nil // Don't fail the task
		}

		if !stakeInfo.Exists {
			log.Debug().Msg("No stake found")
			return nil // Don't fail the task
		}

		log.Debug().Str("amount", stakeInfo.Amount.String()).Msg("Found stake")

		rewardWei := new(big.Float).Mul(
			new(big.Float).SetFloat64(result.Reward),
			new(big.Float).SetFloat64(1e18),
		)
		rewardAmount, _ := rewardWei.Int(nil)

		log.Debug().
			Str("reward", rewardAmount.String()).
			Str("creator", result.CreatorDeviceID).
			Msg("Initiating transfer")

		if err := c.stakeWallet.TransferPayment(nil, result.CreatorAddress, result.DeviceID, rewardAmount); err != nil {
			log.Error().Err(err).
				Str("reward", rewardAmount.String()).
				Str("creator", result.CreatorDeviceID).
				Msg("Transfer failed")
			return fmt.Errorf("reward transfer failed: %w", err)
		}

		log.Info().
			Str("reward", rewardAmount.String()).
			Str("creator", result.CreatorDeviceID).
			Msg("Transfer completed")
		return nil
	}

	// Get private key from keystore
	privateKey, err := keystore.GetPrivateKey()
	if err != nil {
		log.Error().Err(err).Msg("Auth required")
		return fmt.Errorf("auth required: %w", err)
	}

	// Create client with keystore private key
	client, err := wallet.NewClientWithKey(
		c.cfg.Ethereum.RPC,
		big.NewInt(c.cfg.Ethereum.ChainID),
		privateKey,
	)
	if err != nil {
		log.Error().Err(err).
			Str("rpc", c.cfg.Ethereum.RPC).
			Int64("chain", c.cfg.Ethereum.ChainID).
			Msg("Client creation failed")
		return fmt.Errorf("wallet client failed: %w", err)
	}

	log.Debug().
		Str("wallet", client.Address().Hex()).
		Str("rpc", c.cfg.Ethereum.RPC).
		Int64("chain", c.cfg.Ethereum.ChainID).
		Msg("Client initialized")

	stakeWalletAddr := common.HexToAddress(c.cfg.Ethereum.StakeWalletAddress)
	stakeWallet, err := stakewallet.NewStakeWallet(stakeWalletAddr, client)
	if err != nil {
		log.Error().Err(err).
			Str("addr", stakeWalletAddr.Hex()).
			Msg("Contract init failed")
		return fmt.Errorf("stake wallet init failed: %w", err)
	}

	// Check if runner has staked
	stakeInfo, err := stakeWallet.GetStakeInfo(&bind.CallOpts{}, result.DeviceID)
	if err != nil {
		log.Error().Err(err).Msg("Stake info check failed")
		return nil // Don't fail the task
	}

	if !stakeInfo.Exists {
		log.Debug().Msg("No stake found")
		return nil // Don't fail the task
	}

	log.Debug().Str("amount", stakeInfo.Amount.String()).Msg("Found stake")

	// Get transaction options
	txOpts, err := client.GetTransactOpts()
	if err != nil {
		log.Error().Err(err).
			Str("wallet", client.Address().Hex()).
			Msg("TX opts failed")
		return fmt.Errorf("tx opts failed: %w", err)
	}

	rewardWei := new(big.Float).Mul(
		new(big.Float).SetFloat64(result.Reward),
		new(big.Float).SetFloat64(1e18),
	)
	rewardAmount, _ := rewardWei.Int(nil)

	log.Debug().
		Str("reward", rewardAmount.String()).
		Str("creator", result.CreatorDeviceID).
		Msg("Initiating transfer")

	tx, err := stakeWallet.TransferPayment(
		txOpts,
		result.CreatorDeviceID,
		result.DeviceID,
		rewardAmount,
	)
	if err != nil {
		log.Error().Err(err).
			Str("reward", rewardAmount.String()).
			Str("creator", result.CreatorDeviceID).
			Msg("Transfer failed")
		return fmt.Errorf("reward transfer failed: %w", err)
	}

	log.Info().
		Str("tx", tx.Hash().Hex()).
		Str("reward", rewardAmount.String()).
		Msg("Transfer submitted")

	// Wait for confirmation
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		log.Error().Err(err).
			Str("tx", tx.Hash().Hex()).
			Msg("Confirmation failed")
		return fmt.Errorf("confirmation failed: %w", err)
	}

	if receipt.Status == 0 {
		log.Error().
			Str("tx", tx.Hash().Hex()).
			Str("reward", rewardAmount.String()).
			Msg("Transfer reverted")
		return fmt.Errorf("transfer reverted")
	}

	log.Info().
		Str("tx", tx.Hash().Hex()).
		Str("reward", rewardAmount.String()).
		Str("block", receipt.BlockNumber.String()).
		Msg("Transfer confirmed")

	return nil
}
