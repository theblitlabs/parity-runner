package cli

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/utils"
	"github.com/theblitlabs/parity-protocol/pkg/device"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
	"github.com/theblitlabs/parity-protocol/pkg/wallet"
)

func RunBalance() {
	log := logger.Get().With().Str("component", "balance").Logger()

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	privateKey, err := keystore.GetPrivateKey()
	if err != nil {
		log.Fatal().Err(err).Msg("No private key found - please authenticate first using 'parity auth'")
	}

	client, err := wallet.NewClientWithKey(
		cfg.Ethereum.RPC,
		big.NewInt(cfg.Ethereum.ChainID),
		privateKey,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Ethereum client")
	}

	tokenAddr := common.HexToAddress(cfg.Ethereum.TokenAddress)
	stakeWalletAddr := common.HexToAddress(cfg.Ethereum.StakeWalletAddress)

	balance, err := client.GetERC20Balance(tokenAddr, client.Address())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to check token balance")
	}

	log.Info().
		Str("address", client.Address().Hex()).
		Str("balance", utils.FormatEther(balance)+" PRTY").
		Str("token_address", tokenAddr.Hex()).
		Msg("Token balance")

	stakeWallet, err := stakewallet.NewStakeWallet(stakeWalletAddr, client)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create stake wallet")
	}

	deviceID, err := device.VerifyDeviceID()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get device ID")
	}

	stakeInfo, err := stakeWallet.GetStakeInfo(&bind.CallOpts{}, deviceID)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get stake info")
	}

	if stakeInfo.Exists {
		log.Info().
			Str("amount", utils.FormatEther(stakeInfo.Amount)+" PRTY").
			Str("device_id", stakeInfo.DeviceID).
			Msg("Current stake info")
	} else {
		log.Info().Msg("No active stake found")
	}
}
