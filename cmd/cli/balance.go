package cli

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	walletsdk "github.com/theblitlabs/go-wallet-sdk"
	"github.com/theblitlabs/parity-runner/internal/config"

	"os"
	"path/filepath"

	"github.com/theblitlabs/deviceid"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/keystore"
)

func RunBalance() {
	log := gologger.Get().With().Str("component", "balance").Logger()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get user home directory")
	}

	ks, err := keystore.NewKeystore(keystore.Config{
		DirPath:  filepath.Join(homeDir, ".parity"),
		FileName: "keystore.json",
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create keystore")
	}

	privateKey, err := ks.LoadPrivateKey()
	if err != nil {
		log.Fatal().Err(err).Msg("No private key found - please authenticate first using 'parity auth'")
	}

	clientConfig := walletsdk.ClientConfig{
		RPCURL:       cfg.Ethereum.RPC,
		ChainID:      cfg.Ethereum.ChainID,
		PrivateKey:   common.Bytes2Hex(crypto.FromECDSA(privateKey)),
		TokenAddress: common.HexToAddress(cfg.Ethereum.TokenAddress),
		StakeAddress: common.HexToAddress(cfg.Ethereum.StakeWalletAddress),
	}

	client, err := walletsdk.NewClient(clientConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Ethereum client")
	}

	walletBalance, err := client.GetBalance(client.Address())
	if err != nil {
		select {
		case <-ctx.Done():
			log.Fatal().Err(ctx.Err()).Msg("Operation timed out while getting wallet balance")
		default:
			log.Fatal().Err(err).Msg("Failed to get wallet balance")
		}
	}

	log.Info().
		Str("wallet_address", client.Address().Hex()).
		Str("balance", walletBalance.String()+" PRTY").
		Msg("Wallet token balance")

	deviceIDManager := deviceid.NewManager(deviceid.Config{})
	deviceID, err := deviceIDManager.VerifyDeviceID()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get device ID")
	}

	stakeInfo, err := client.GetStakeInfo(deviceID)
	if err != nil {
		select {
		case <-ctx.Done():
			log.Fatal().Err(ctx.Err()).Msg("Operation timed out while getting stake info")
		default:
			log.Fatal().Err(err).Msg("Failed to get stake info")
		}
	}

	if stakeInfo.Exists {
		log.Info().
			Str("amount", stakeInfo.Amount.String()+" PRTY").
			Str("device_id", stakeInfo.DeviceID).
			Str("wallet_address", stakeInfo.WalletAddress.Hex()).
			Msg("Current stake info")

		contractBalance, err := client.GetBalance(clientConfig.StakeAddress)
		if err != nil {
			select {
			case <-ctx.Done():
				log.Fatal().Err(ctx.Err()).Msg("Operation timed out while getting contract balance")
			default:
				log.Fatal().Err(err).Msg("Failed to get contract balance")
			}
		}
		log.Info().
			Str("balance", contractBalance.String()).
			Str("contract_address", clientConfig.StakeAddress.Hex()).
			Msg("Contract token balance")
	} else {
		log.Info().Msg("No active stake found")
	}
}
