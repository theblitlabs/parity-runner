package auth

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
	"github.com/theblitlabs/parity-protocol/pkg/wallet"
)

func Run() {
	var privateKey string

	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with the network",
		Run: func(cmd *cobra.Command, args []string) {
			executeAuth(privateKey)
		},
	}

	cmd.Flags().StringVarP(&privateKey, "private-key", "k", "", "Private key in hex format")
	cmd.MarkFlagRequired("private-key")
}

func executeAuth(privateKey string) {
	log := log.With().Str("component", "auth").Logger()

	if privateKey == "" {
		log.Fatal().Msg("Private key is required")
	}

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	if err := keystore.SavePrivateKey(privateKey); err != nil {
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
