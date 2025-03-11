package cli

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

func (c *EthereumRewardClient) SetStakeWallet(sw StakeWallet) {
	c.stakeWallet = sw
}

func (c *EthereumRewardClient) DistributeRewards(result *models.TaskResult) error {
	log := logger.WithComponent("rewards").With().
		Str("task", result.TaskID.String()).
		Str("device", result.DeviceID).
		Logger()

	log.Info().Msg("Starting reward distribution")

	if result.Reward <= 0 {
		log.Error().Msg("Invalid reward amount - must be greater than zero")
		return fmt.Errorf("invalid reward amount: must be greater than zero")
	}

	if c.stakeWallet != nil {
		stakeInfo, err := c.stakeWallet.GetStakeInfo(&bind.CallOpts{}, result.DeviceID)
		if err != nil {
			log.Error().Err(err).Msg("Stake info check failed")
			return nil
		}

		if !stakeInfo.Exists {
			log.Debug().Msg("No stake found")
			return nil
		}

		rewardWei := new(big.Float).Mul(
			new(big.Float).SetFloat64(result.Reward),
			new(big.Float).SetFloat64(1e18),
		)
		rewardAmount, _ := rewardWei.Int(nil)

		if err := c.stakeWallet.TransferPayment(nil, result.CreatorAddress, result.DeviceID, rewardAmount); err != nil {
			log.Error().Err(err).
				Str("reward", rewardAmount.String()).
				Msg("Transfer failed")
			return fmt.Errorf("reward transfer failed: %w", err)
		}

		log.Info().Str("reward", rewardAmount.String()).Msg("Transfer completed")
		return nil
	}

	privateKey, err := keystore.GetPrivateKey()
	if err != nil {
		log.Error().Err(err).Msg("Auth required")
		return fmt.Errorf("auth required: %w", err)
	}

	client, err := wallet.NewClientWithKey(
		c.cfg.Ethereum.RPC,
		big.NewInt(c.cfg.Ethereum.ChainID),
		privateKey,
	)
	if err != nil {
		log.Error().Err(err).Msg("Client creation failed")
		return fmt.Errorf("wallet client failed: %w", err)
	}

	stakeWalletAddr := common.HexToAddress(c.cfg.Ethereum.StakeWalletAddress)
	stakeWallet, err := stakewallet.NewStakeWallet(stakeWalletAddr, client)
	if err != nil {
		log.Error().Err(err).Msg("Contract init failed")
		return fmt.Errorf("stake wallet init failed: %w", err)
	}

	stakeInfo, err := stakeWallet.GetStakeInfo(&bind.CallOpts{}, result.DeviceID)
	if err != nil {
		log.Error().Err(err).Msg("Stake info check failed")
		return nil
	}

	if !stakeInfo.Exists {
		log.Debug().Msg("No stake found")
		return nil
	}

	txOpts, err := client.GetTransactOpts()
	if err != nil {
		log.Error().Err(err).Msg("TX opts failed")
		return fmt.Errorf("tx opts failed: %w", err)
	}

	rewardWei := new(big.Float).Mul(
		new(big.Float).SetFloat64(result.Reward),
		new(big.Float).SetFloat64(1e18),
	)
	rewardAmount, _ := rewardWei.Int(nil)

	tx, err := stakeWallet.TransferPayment(
		txOpts,
		result.CreatorDeviceID,
		result.DeviceID,
		rewardAmount,
	)
	if err != nil {
		log.Error().Err(err).
			Str("reward", rewardAmount.String()).
			Msg("Transfer failed")
		return fmt.Errorf("reward transfer failed: %w", err)
	}

	log.Info().Str("tx", tx.Hash().Hex()).Msg("Transfer submitted")

	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		log.Error().Err(err).Str("tx", tx.Hash().Hex()).Msg("Confirmation failed")
		return fmt.Errorf("confirmation failed: %w", err)
	}

	if receipt.Status == 0 {
		log.Error().Str("tx", tx.Hash().Hex()).Msg("Transfer reverted")
		return fmt.Errorf("transfer reverted")
	}

	log.Info().
		Str("tx", tx.Hash().Hex()).
		Str("block", receipt.BlockNumber.String()).
		Msg("Transfer confirmed")

	return nil
}
