package stake

import (
	"flag"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/virajbhartiya/parity-protocol/internal/config"
	"github.com/virajbhartiya/parity-protocol/pkg/device"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
	"github.com/virajbhartiya/parity-protocol/pkg/stakewallet"
	"github.com/virajbhartiya/parity-protocol/pkg/wallet"
)

func Run() {
	log := logger.Get()

	stakeFlags := flag.NewFlagSet("stake", flag.ExitOnError)
	amountFlag := stakeFlags.Float64("amount", 1.0, "Amount of PRTY tokens to stake")

	if err := stakeFlags.Parse(os.Args[2:]); err != nil {
		log.Fatal().Err(err).Msg("Failed to parse flags")
	}

	amount := new(big.Float).Mul(
		new(big.Float).SetFloat64(*amountFlag),
		new(big.Float).SetFloat64(1e18),
	)
	amountWei, _ := amount.Int(nil)

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

	if balance.Cmp(amountWei) < 0 {
		log.Fatal().
			Str("balance", balance.String()).
			Str("required", amountWei.String()).
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

	if allowance.Cmp(amountWei) < 0 {
		txOpts, err := client.GetTransactOpts()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to get transaction options")
		}

		tx, err := client.ApproveToken(txOpts, tokenAddr, stakeWalletAddr, amountWei)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to approve token spending")
		}

		log.Info().
			Str("tx_hash", tx.Hash().String()).
			Str("amount", amountWei.String()).
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

	tx, err := stakeWallet.Stake(txOpts, amountWei, deviceID)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to stake tokens")
	}

	log.Info().
		Str("tx_hash", tx.Hash().String()).
		Str("amount", formatEther(amountWei)+" PRTY").
		Str("device_id", deviceID).
		Msg("Staking transaction sent - waiting for confirmation...")
}

func formatEther(wei *big.Int) string {
	ether := new(big.Float).Quo(
		new(big.Float).SetInt(wei),
		new(big.Float).SetFloat64(1e18),
	)
	return ether.Text('f', 2)
}
