package cli

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/pkg/wallet"
)

type KeyStore struct {
	PrivateKey string `json:"private_key"`
}

func RunAuth() {
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

	if err := cmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Failed to execute auth command")
	}
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

	// Validate and normalize private key format
	privateKey = strings.TrimPrefix(privateKey, "0x")
	if len(privateKey) != 64 {
		log.Fatal().Msg("Invalid private key - must be 64 hex characters without 0x prefix")
	}
	if _, err := crypto.HexToECDSA(privateKey); err != nil {
		log.Fatal().Err(err).Msg("Invalid private key format")
	}

	// 1. First create the keystore directory
	keystoreDir := filepath.Join(os.Getenv("HOME"), ".parity")
	if err := os.MkdirAll(keystoreDir, 0700); err != nil {
		log.Fatal().Err(err).Msg("Failed to create keystore directory")
	}

	// 2. Save private key to keystore first
	keystorePath := filepath.Join(keystoreDir, "keystore.json")
	keystore := KeyStore{
		PrivateKey: privateKey,
	}
	keystoreData, err := json.MarshalIndent(keystore, "", "  ")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to marshal keystore data")
	}

	if err := os.WriteFile(keystorePath, keystoreData, 0600); err != nil {
		log.Fatal().Err(err).Msg("Failed to save keystore")
	}

	// 3. Then validate by creating client
	client, err := wallet.NewClientWithKey(
		cfg.Ethereum.RPC,
		big.NewInt(cfg.Ethereum.ChainID),
		privateKey,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid private key")
	}

	log.Info().
		Str("address", client.Address().Hex()).
		Str("keystore", keystorePath).
		Msg("Wallet authenticated successfully")
}
