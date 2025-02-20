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
	log := log.With().
		Str("component", "reward_client").
		Str("task_id", result.TaskID.String()).
		Str("device_id", result.DeviceID).
		Logger()

	log.Info().Msg("Starting reward distribution")

	// If we have a mock stake wallet (for testing), use it
	if c.stakeWallet != nil {
		stakeInfo, err := c.stakeWallet.GetStakeInfo(&bind.CallOpts{}, result.DeviceID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to check stake info")
			return nil // Don't fail the task
		}

		if !stakeInfo.Exists {
			log.Error().Msg("No stake found for runner - staking required")
			return nil // Don't fail the task
		}

		log.Info().
			Str("stake_amount", stakeInfo.Amount.String()).
			Msg("Found valid stake")

		rewardWei := new(big.Float).Mul(
			new(big.Float).SetFloat64(result.Reward),
			new(big.Float).SetFloat64(1e18),
		)
		rewardAmount, _ := rewardWei.Int(nil)

		log.Info().
			Str("reward_amount", rewardAmount.String()).
			Str("creator_address", result.CreatorAddress).
			Msg("Transferring reward payment")

		if err := c.stakeWallet.TransferPayment(nil, result.CreatorAddress, result.DeviceID, rewardAmount); err != nil {
			log.Error().Err(err).Msg("Failed to transfer reward payment")
			return fmt.Errorf("failed to distribute rewards: %w", err)
		}

		log.Info().Msg("Reward payment transferred successfully")
		return nil
	}

	// Get private key from keystore
	privateKey, err := keystore.GetPrivateKey()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get private key")
		return fmt.Errorf("no private key found - please authenticate first: %w", err)
	}

	// Create client with keystore private key
	client, err := wallet.NewClientWithKey(
		c.cfg.Ethereum.RPC,
		big.NewInt(c.cfg.Ethereum.ChainID),
		privateKey,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create wallet client")
		return fmt.Errorf("failed to create wallet client: %w", err)
	}

	log.Info().
		Str("wallet_address", client.Address().Hex()).
		Msg("Wallet client initialized")

	stakeWalletAddr := common.HexToAddress(c.cfg.Ethereum.StakeWalletAddress)
	stakeWallet, err := stakewallet.NewStakeWallet(stakeWalletAddr, client)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create stake wallet")
		return fmt.Errorf("failed to create stake wallet: %w", err)
	}

	// Check if runner has staked
	stakeInfo, err := stakeWallet.GetStakeInfo(&bind.CallOpts{}, result.DeviceID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check stake info")
		return nil // Don't fail the task
	}

	if !stakeInfo.Exists {
		log.Error().Msg("No stake found for runner - please stake tokens first using 'parity stake'")
		return nil // Don't fail the task
	}

	log.Info().
		Str("stake_amount", stakeInfo.Amount.String()).
		Msg("Found valid stake")

	// Get transaction options from the authenticated client
	txOpts, err := client.GetTransactOpts()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get transaction options")
		return fmt.Errorf("failed to get transaction options: %w", err)
	}

	rewardWei := new(big.Float).Mul(
		new(big.Float).SetFloat64(result.Reward),
		new(big.Float).SetFloat64(1e18),
	)
	rewardAmount, _ := rewardWei.Int(nil)

	log.Info().
		Str("reward_amount", rewardAmount.String()).
		Str("creator_address", result.CreatorAddress).
		Msg("Transferring reward payment")

	tx, err := stakeWallet.TransferPayment(
		txOpts,
		result.CreatorAddress,
		result.DeviceID,
		rewardAmount,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to transfer reward payment")
		return fmt.Errorf("failed to distribute rewards: %w", err)
	}

	log.Info().
		Str("tx_hash", tx.Hash().Hex()).
		Msg("Reward payment transaction submitted")

	// Wait for transaction confirmation
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to confirm reward payment")
		return fmt.Errorf("failed to confirm reward distribution: %w", err)
	}

	if receipt.Status == 0 {
		log.Error().Msg("Reward payment transaction failed")
		return fmt.Errorf("reward distribution transaction failed")
	}

	log.Info().
		Str("tx_hash", tx.Hash().Hex()).
		Msg("Reward payment confirmed successfully")

	return nil
}
