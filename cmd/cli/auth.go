package cli

import (
	"encoding/json"
	"fmt"
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
			if err := ExecuteAuth(privateKey, "config/config.yaml"); err != nil {
				log.Fatal().Err(err).Msg("Failed to authenticate")
			}
		},
	}

	cmd.Flags().StringVarP(&privateKey, "private-key", "k", "", "Private key in hex format")
	if err := cmd.MarkFlagRequired("private-key"); err != nil {
		log.Fatal().Err(err).Msg("Failed to mark flag as required")
	}

	if err := cmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Failed to execute auth command")
	}
}

func ExecuteAuth(privateKey string, configPath string) error {
	log := log.With().Str("component", "auth").Logger()

	if privateKey == "" {
		return fmt.Errorf("private key is required")
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	privateKey = strings.TrimPrefix(privateKey, "0x")

	if _, err := crypto.HexToECDSA(privateKey); err != nil {
		return fmt.Errorf("invalid private key format: %w", err)
	}

	if len(privateKey) != 64 {
		return fmt.Errorf("invalid private key - must be 64 hex characters without 0x prefix")
	}

	keystoreDir := filepath.Join(os.Getenv("HOME"), ".parity")
	if err := os.MkdirAll(keystoreDir, 0700); err != nil {
		return fmt.Errorf("failed to create keystore directory: %w", err)
	}

	keystorePath := filepath.Join(keystoreDir, "keystore.json")
	keystore := KeyStore{
		PrivateKey: privateKey,
	}
	keystoreData, err := json.MarshalIndent(keystore, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal keystore data: %w", err)
	}

	if err := os.WriteFile(keystorePath, keystoreData, 0600); err != nil {
		return fmt.Errorf("failed to save keystore: %w", err)
	}

	client, err := wallet.NewClientWithKey(
		cfg.Ethereum.RPC,
		big.NewInt(cfg.Ethereum.ChainID),
		privateKey,
	)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	log.Info().
		Str("address", client.Address().Hex()).
		Str("keystore", keystorePath).
		Msg("Wallet authenticated successfully")

	return nil
}
