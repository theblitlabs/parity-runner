package auth

import (
	"flag"
	"os"

	"github.com/virajbhartiya/parity-protocol/internal/config"
	"github.com/virajbhartiya/parity-protocol/pkg/keystore"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
	"github.com/virajbhartiya/parity-protocol/pkg/wallet"
)

func Run() {
	log := logger.Get()

	authFlags := flag.NewFlagSet("auth", flag.ExitOnError)
	privateKeyHex := authFlags.String("private-key", "", "Private key in hex format")

	if err := authFlags.Parse(os.Args[2:]); err != nil {
		log.Fatal().Err(err).Msg("Failed to parse flags")
	}

	if *privateKeyHex == "" {
		log.Fatal().Msg("Private key is required. Use --private-key flag")
	}

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	if err := keystore.SavePrivateKey(*privateKeyHex); err != nil {
		log.Fatal().Err(err).Msg("Failed to save private key")
	}

	client, err := wallet.NewClient(cfg.Ethereum.RPC, cfg.Ethereum.ChainID)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Ethereum network")
	}

	log.Info().
		Str("address", client.Address().Hex()).
		Msg("Wallet authenticated successfully")
}
