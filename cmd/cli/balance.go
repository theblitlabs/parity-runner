package cli

import (
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	paritywallet "github.com/theblitlabs/go-parity-wallet"
	"github.com/theblitlabs/parity-runner/internal/config"

	"github.com/theblitlabs/deviceid"
	stakeclient "github.com/theblitlabs/go-stake-client"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/keystore"
)

func RunBalance() {
	log := gologger.Get().With().Str("component", "balance").Logger()

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	ks, err := keystore.NewKeystore(keystore.Config{})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create keystore")
	}

	privateKey, err := ks.LoadPrivateKey()
	if err != nil {
		log.Fatal().Err(err).Msg("No private key found - please authenticate first using 'parity auth'")
	}

	client, err := paritywallet.NewClientWithKey(
		cfg.Ethereum.RPC,
		cfg.Ethereum.ChainID,
		common.Bytes2Hex(crypto.FromECDSA(privateKey)),
		crypto.PubkeyToAddress(privateKey.PublicKey),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Ethereum client")
	}

	tokenAddr := common.HexToAddress(cfg.Ethereum.TokenAddress)
	stakeWalletAddr := common.HexToAddress(cfg.Ethereum.StakeWalletAddress)

	token, err := paritywallet.NewParityToken(tokenAddr, client)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create token contract")
	}

	balance, err := token.BalanceOf(&bind.CallOpts{}, client.Address())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to check token balance")
	}

	log.Info().
		Str("address", client.Address().Hex()).
		Str("balance", balance.String()).
		Str("token_address", tokenAddr.Hex()).
		Msg("Token balance")

	stakeWallet, err := stakeclient.NewStakeWallet(client, tokenAddr, stakeWalletAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create stake wallet")
	}

	deviceIDManager := deviceid.NewManager(deviceid.Config{})
	deviceID, err := deviceIDManager.VerifyDeviceID()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get device ID")
	}

	stakeInfo, err := stakeWallet.GetStakeInfo(deviceID)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get stake info")
	}

	if stakeInfo.Exists {
		log.Info().
			Str("amount", stakeInfo.Amount.String()+" PRTY").
			Str("device_id", stakeInfo.DeviceID).
			Msg("Current stake info")
	} else {
		log.Info().Msg("No active stake found")
	}
}
