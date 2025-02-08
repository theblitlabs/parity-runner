package stake

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/pkg/device"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
	"github.com/theblitlabs/parity-protocol/pkg/wallet"
)

func Run() {
	var amount float64

	cmd := &cobra.Command{
		Use:   "stake",
		Short: "Stake tokens in the network",
		Run: func(cmd *cobra.Command, args []string) {
			executeStake(amount)
		},
	}

	cmd.Flags().Float64VarP(&amount, "amount", "a", 1.0, "Amount of PRTY tokens to stake")
	cmd.MarkFlagRequired("amount")
}

func executeStake(amount float64) {
	log := log.With().Str("component", "stake").Logger()

	// Convert amount to wei
	amountWei := new(big.Float).Mul(
		big.NewFloat(amount),
		new(big.Float).SetFloat64(1e18),
	)
	wei, _ := amountWei.Int(nil)

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	client, err := wallet.NewClient(cfg.Ethereum.RPC, cfg.Ethereum.ChainID)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Ethereum client")
	}

	deviceID, err := device.VerifyDeviceID()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to verify device ID")
	}

	tokenAddr := common.HexToAddress(cfg.Ethereum.TokenAddress)
	stakeWalletAddr := common.HexToAddress(cfg.Ethereum.StakeWalletAddress)

	balance, err := client.GetERC20Balance(tokenAddr, client.Address())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to check token balance")
	}

	if balance.Cmp(wei) < 0 {
		log.Fatal().
			Str("balance", balance.String()).
			Str("required", wei.String()).
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

	if allowance.Cmp(wei) < 0 {
		txOpts, err := client.GetTransactOpts()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to get transaction options")
		}

		tx, err := client.ApproveToken(txOpts, tokenAddr, stakeWalletAddr, wei)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to approve token spending")
		}

		log.Info().
			Str("tx_hash", tx.Hash().String()).
			Str("amount", wei.String()).
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

	tx, err := stakeWallet.Stake(txOpts, wei, deviceID, client.Address())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to stake tokens")
	}

	log.Info().
		Str("tx_hash", tx.Hash().String()).
		Str("amount", formatEther(wei)+" PRTY").
		Str("device_id", deviceID).
		Str("wallet_address", client.Address().Hex()).
		Msg("Tokens staked successfully")

	log.Info().
		Str("device_id", deviceID).
		Str("wallet_address", client.Address().Hex()).
		Msg("Device wallet registered successfully")
}

func formatEther(wei *big.Int) string {
	ether := new(big.Float).Quo(
		new(big.Float).SetInt(wei),
		new(big.Float).SetFloat64(1e18),
	)
	return ether.Text('f', 2)
}
