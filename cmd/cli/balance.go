package cli

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"github.com/theblitlabs/gologger"

	"github.com/theblitlabs/parity-runner/internal/utils"
)

func RunBalance() {
	logger := gologger.Get().With().Str("component", "balance").Logger()

	cmd := utils.CreateCommand(utils.CommandConfig{
		Use:   "balance",
		Short: "Check token balances and stake status",
		RunFunc: func(cmd *cobra.Command, args []string) error {
			return executeBalance()
		},
	}, logger)

	utils.ExecuteCommand(cmd, logger)
}

func executeBalance() error {
	logger := gologger.Get().With().Str("component", "balance").Logger()

	ctx, cancel := utils.WithTimeout()
	defer cancel()

	cfg, err := utils.GetConfig()
	if err != nil {
		return err
	}

	client, err := utils.NewClient(cfg)
	if err != nil {
		return err
	}

	walletBalance, err := client.GetBalance(client.Address())
	if err != nil {
		utils.HandleContextFatal(logger, ctx, err,
			"Operation timed out while getting wallet balance",
			"Failed to get wallet balance")
		return err
	}

	logger.Info().
		Str("wallet_address", client.Address().Hex()).
		Str("balance", walletBalance.String()+" PRTY").
		Msg("Wallet token balance")

	deviceID, err := utils.GetDeviceID()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to get device ID")
		return err
	}

	stakeInfo, err := client.GetStakeInfo(deviceID)
	if err != nil {
		utils.HandleContextFatal(logger, ctx, err,
			"Operation timed out while getting stake info",
			"Failed to get stake info")
		return err
	}

	if stakeInfo.Exists {
		logger.Info().
			Str("amount", stakeInfo.Amount.String()+" PRTY").
			Str("device_id", stakeInfo.DeviceID).
			Str("wallet_address", stakeInfo.WalletAddress.Hex()).
			Msg("Current stake info")

		stakeAddress := common.HexToAddress(cfg.FilecoinNetwork.StakeWalletAddress)
		contractBalance, err := client.GetBalance(stakeAddress)
		if err != nil {
			utils.HandleContextFatal(logger, ctx, err,
				"Operation timed out while getting contract balance",
				"Failed to get contract balance")
			return err
		}
		logger.Info().
			Str("balance", contractBalance.String()).
			Str("contract_address", stakeAddress.Hex()).
			Msg("Contract token balance")
	} else {
		logger.Info().Msg("No active stake found")
	}

	return nil
}
