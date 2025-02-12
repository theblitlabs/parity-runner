package runner

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog/log"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
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
	ctx := context.Background()
	log.Info().Msg("Checking stake status before distributing rewards")

	// If we have a mock stake wallet (for testing), use it
	if c.stakeWallet != nil {
		stakeInfo, err := c.stakeWallet.GetStakeInfo(&bind.CallOpts{}, result.DeviceID)
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

		rewardWei := new(big.Float).Mul(
			new(big.Float).SetFloat64(result.Reward),
			new(big.Float).SetFloat64(1e18),
		)
		rewardAmount, _ := rewardWei.Int(nil)

		if err := c.stakeWallet.TransferPayment(nil, result.CreatorAddress, result.DeviceID, rewardAmount); err != nil {
			return fmt.Errorf("failed to distribute rewards: %w", err)
		}

		return nil
	}

	// Get private key from keystore
	privateKey, err := keystore.GetPrivateKey()
	if err != nil {
		return fmt.Errorf("no private key found - please authenticate first: %w", err)
	}

	// Create client with keystore private key
	client, err := wallet.NewClientWithKey(
		c.cfg.Ethereum.RPC,
		big.NewInt(c.cfg.Ethereum.ChainID),
		privateKey,
	)
	if err != nil {
		return fmt.Errorf("failed to create wallet client: %w", err)
	}

	stakeWalletAddr := common.HexToAddress(c.cfg.Ethereum.StakeWalletAddress)
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

	// Get transaction options from the authenticated client
	txOpts, err := client.GetTransactOpts()
	if err != nil {
		return fmt.Errorf("failed to get transaction options: %w", err)
	}

	rewardWei := new(big.Float).Mul(
		new(big.Float).SetFloat64(result.Reward),
		new(big.Float).SetFloat64(1e18),
	)
	rewardAmount, _ := rewardWei.Int(nil)

	tx, err := stakeWallet.TransferPayment(
		txOpts,
		result.CreatorAddress,
		result.DeviceID,
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
