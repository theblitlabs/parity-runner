package cli

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"
	walletsdk "github.com/theblitlabs/go-wallet-sdk"
	"github.com/theblitlabs/gologger"

	"github.com/theblitlabs/parity-runner/internal/utils"
)

func RunStake() {
	var amount float64

	logger := gologger.Get().With().Str("component", "stake").Logger()
	logger.Info().Msg("Starting staking process...")

	cmd := utils.CreateCommand(utils.CommandConfig{
		Use:   "stake",
		Short: "Stake tokens in the network",
		Flags: map[string]utils.Flag{
			"amount": {
				Type:           utils.FlagTypeFloat64,
				Shorthand:      "a",
				Description:    "Amount of PRTY tokens to stake",
				DefaultFloat64: 1.0,
				Required:       true,
			},
		},
		RunFunc: func(cmd *cobra.Command, args []string) error {
			var err error
			amount, err = cmd.Flags().GetFloat64("amount")
			if err != nil {
				return err
			}

			logger.Info().
				Float64("amount", amount).
				Msg("Processing stake request")

			return executeStake(amount)
		},
	}, logger)

	utils.ExecuteCommand(cmd, logger)
}

func executeStake(amount float64) error {
	logger := gologger.Get().With().Str("component", "stake").Logger()

	cfg, err := utils.GetConfig()
	if err != nil {
		logger.Fatal().
			Err(err).
			Msg("Failed to load configuration - please ensure config.yaml exists")
		return err
	}

	client, err := utils.NewClient(cfg)
	if err != nil {
		logger.Fatal().
			Err(err).
			Msg("Failed to create wallet client")
		return err
	}

	deviceID, err := utils.GetDeviceID()
	if err != nil {
		logger.Fatal().
			Err(err).
			Msg("Failed to verify device - please ensure you have a valid device ID")
		return err
	}

	logger.Info().
		Str("device_id", deviceID).
		Str("wallet", client.Address().Hex()).
		Msg("Device verified successfully")

	tokenAddr := common.HexToAddress(cfg.Ethereum.TokenAddress)
	stakeWalletAddr := common.HexToAddress(cfg.Ethereum.StakeWalletAddress)

	token, err := walletsdk.NewParityToken(tokenAddr, client)
	if err != nil {
		logger.Fatal().
			Err(err).
			Str("token_address", tokenAddr.Hex()).
			Str("wallet", client.Address().Hex()).
			Msg("Failed to create token contract - please try again")
		return err
	}

	balance, err := token.BalanceOf(&bind.CallOpts{}, client.Address())
	if err != nil {
		logger.Fatal().
			Err(err).
			Str("token_address", tokenAddr.Hex()).
			Str("wallet", client.Address().Hex()).
			Msg("Failed to check token balance - please try again")
		return err
	}

	amountToStake := amountWei(amount)
	if balance.Cmp(amountToStake) < 0 {
		logger.Fatal().
			Str("current_balance", utils.FormatEther(balance)+" PRTY").
			Str("required_amount", utils.FormatEther(amountToStake)+" PRTY").
			Msg("Insufficient token balance - please ensure you have enough PRTY tokens")
		return err
	}

	logger.Info().
		Str("balance", utils.FormatEther(balance)+" PRTY").
		Str("amount_to_stake", utils.FormatEther(amountToStake)+" PRTY").
		Msg("Sufficient balance found")

	allowance, err := token.Allowance(&bind.CallOpts{}, client.Address(), stakeWalletAddr)
	if err != nil {
		logger.Fatal().
			Err(err).
			Str("token_address", tokenAddr.Hex()).
			Str("stake_wallet", stakeWalletAddr.Hex()).
			Msg("Failed to check token allowance - please try again")
		return err
	}

	if allowance.Cmp(amountToStake) < 0 {
		logger.Info().
			Str("amount", utils.FormatEther(amountToStake)+" PRTY").
			Msg("Approving token spending...")

		txOpts, err := client.GetTransactOpts()
		if err != nil {
			logger.Fatal().
				Err(err).
				Msg("Failed to prepare transaction - please try again")
			return err
		}

		tx, err := token.Approve(txOpts, stakeWalletAddr, amountToStake)
		if err != nil {
			logger.Fatal().
				Err(err).
				Str("amount", utils.FormatEther(amountToStake)+" PRTY").
				Msg("Failed to approve token spending - please try again")
			return err
		}

		logger.Info().
			Str("tx_hash", tx.Hash().Hex()).
			Str("amount", utils.FormatEther(amountToStake)+" PRTY").
			Msg("Token approval submitted - waiting for confirmation...")

		receipt, err := bind.WaitMined(context.Background(), client, tx)
		if err != nil {
			logger.Fatal().
				Err(err).
				Str("tx_hash", tx.Hash().Hex()).
				Msg("Failed to confirm token approval - please check the transaction status")
			return err
		}

		if receipt.Status == 0 {
			logger.Fatal().
				Str("tx_hash", tx.Hash().Hex()).
				Msg("Token approval failed - please check the transaction status")
			return err
		}

		logger.Info().
			Str("tx_hash", tx.Hash().Hex()).
			Msg("Token approval confirmed successfully")

		time.Sleep(5 * time.Second)
	}

	stakeWallet, err := walletsdk.NewStakeWallet(client, stakeWalletAddr, tokenAddr)
	if err != nil {
		logger.Fatal().
			Err(err).
			Str("stake_wallet", stakeWalletAddr.Hex()).
			Msg("Failed to connect to stake contract - please try again")
		return err
	}

	logger.Info().
		Str("amount", utils.FormatEther(amountToStake)+" PRTY").
		Str("device_id", deviceID).
		Msg("Submitting stake transaction...")

	tx, err := stakeWallet.Stake(amountToStake, deviceID)
	if err != nil {
		logger.Fatal().
			Err(err).
			Str("amount", utils.FormatEther(amountToStake)+" PRTY").
			Str("device_id", deviceID).
			Msg("Failed to submit stake transaction - please try again")
		return err
	}

	logger.Info().
		Str("tx_hash", tx.Hash().Hex()).
		Str("amount", utils.FormatEther(amountToStake)+" PRTY").
		Str("device_id", deviceID).
		Str("wallet", client.Address().Hex()).
		Msg("Stake transaction submitted - waiting for confirmation...")

	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		logger.Error().
			Err(err).
			Str("tx_hash", tx.Hash().Hex()).
			Msg("Failed to confirm stake transaction - please check the transaction status")
		return err
	}

	if receipt.Status == 1 {
		logger.Info().
			Str("tx_hash", tx.Hash().Hex()).
			Str("amount", utils.FormatEther(amountToStake)+" PRTY").
			Str("device_id", deviceID).
			Str("wallet", client.Address().Hex()).
			Uint64("block_number", receipt.BlockNumber.Uint64()).
			Msg("Stake transaction confirmed successfully! Your device is now registered and ready to process tasks.")
	} else {
		logger.Error().
			Str("tx_hash", tx.Hash().Hex()).
			Str("amount", utils.FormatEther(amountToStake)+" PRTY").
			Msg("Stake transaction failed - please check the transaction status")
	}

	return nil
}

func amountWei(amount float64) *big.Int {
	amountWei := new(big.Float).Mul(
		big.NewFloat(amount),
		new(big.Float).SetFloat64(1e18),
	)
	wei, _ := amountWei.Int(nil)
	return wei
}
