package stake

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/pkg/device"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
	"github.com/theblitlabs/parity-protocol/pkg/wallet"
)

func Run() {
	var amount float64

	log := logger.Get()
	log.Info().Msg("Starting stake process...")

	cmd := &cobra.Command{
		Use:   "stake",
		Short: "Stake tokens in the network",
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Float64("amount", amount).Msg("Received stake command")
			executeStake(amount)
		},
	}

	cmd.Flags().Float64VarP(&amount, "amount", "a", 1.0, "Amount of PRTY tokens to stake")
	cmd.MarkFlagRequired("amount")

	if err := cmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Failed to execute stake command")
	}
}

func executeStake(amount float64) {
	log := logger.Get()
	log.Info().Float64("amount", amount).Msg("Processing stake request")

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Error().Err(err).Msg("Failed to load config")
		return
	}

	// Get private key from keystore
	privateKey, err := keystore.GetPrivateKey()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get private key")
		return
	}

	// Create Ethereum client
	client, err := wallet.NewClientWithKey(
		cfg.Ethereum.RPC,
		big.NewInt(cfg.Ethereum.ChainID),
		privateKey,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Ethereum client")
		return
	}

	deviceID, err := device.VerifyDeviceID()
	log.Info().Str("device_id", deviceID).Msg("Device verified")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to verify device ID")
	}

	tokenAddr := common.HexToAddress(cfg.Ethereum.TokenAddress)
	stakeWalletAddr := common.HexToAddress(cfg.Ethereum.StakeWalletAddress)

	balance, err := client.GetERC20Balance(tokenAddr, client.Address())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to check token balance")
	}

	if balance.Cmp(amountWei(amount)) < 0 {
		log.Fatal().
			Str("balance", balance.String()).
			Str("required", amountWei(amount).String()).
			Msg("Insufficient token balance")
	}

	log.Info().
		Str("balance", balance.String()).
		Str("token_address", tokenAddr.Hex()).
		Msg("Current token balance")

	// Check allowance
	allowance, err := client.GetAllowance(tokenAddr, client.Address(), stakeWalletAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to check allowance")
	}

	if allowance.Cmp(amountWei(amount)) < 0 {
		txOpts, err := client.GetTransactOpts()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to get transaction options")
		}

		tx, err := client.ApproveToken(txOpts, tokenAddr, stakeWalletAddr, amountWei(amount))
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to approve token spending")
		}

		log.Info().
			Str("tx_hash", tx.Hash().String()).
			Str("amount", amountWei(amount).String()).
			Msg("Token approval transaction sent - waiting for confirmation...")

		time.Sleep(15 * time.Second) // TODO: Check is the approval tx is mined on chain
	}

	stakeWallet, err := stakewallet.NewStakeWallet(stakeWalletAddr, client)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create stake wallet")
	}

	txOpts, err := client.GetTransactOpts()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get transaction options")
	}

	tx, err := stakeWallet.Stake(txOpts, amountWei(amount), deviceID, client.Address())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to stake tokens")
	}

	log.Info().
		Str("tx_hash", tx.Hash().String()).
		Str("amount", formatEther(amountWei(amount))+" PRTY").
		Str("device_id", deviceID).
		Str("wallet_address", client.Address().Hex()).
		Msg("Tokens staked successfully")

	log.Info().
		Str("device_id", deviceID).
		Str("wallet_address", client.Address().Hex()).
		Msg("Device wallet registered successfully")

	// Wait for transaction confirmation
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		log.Error().Err(err).Msg("Error waiting for transaction confirmation")
		return
	}

	if receipt.Status == 1 {
		log.Info().
			Str("tx_hash", tx.Hash().Hex()).
			Msg("Stake transaction confirmed successfully")
	} else {
		log.Error().Msg("Stake transaction failed in block")
	}
}

func formatEther(wei *big.Int) string {
	ether := new(big.Float).Quo(
		new(big.Float).SetInt(wei),
		new(big.Float).SetFloat64(1e18),
	)
	return ether.Text('f', 2)
}

func amountWei(amount float64) *big.Int {
	amountWei := new(big.Float).Mul(
		big.NewFloat(amount),
		new(big.Float).SetFloat64(1e18),
	)
	wei, _ := amountWei.Int(nil)
	return wei
}
