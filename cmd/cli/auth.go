package cli

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/theblitlabs/parity-runner/internal/utils"
)

func RunAuth() {
	var privateKey string
	logger := log.With().Str("component", "auth").Logger()

	cmd := utils.CreateCommand(utils.CommandConfig{
		Use:   "auth",
		Short: "Authenticate with the network",
		Flags: map[string]utils.Flag{
			"private-key": {
				Type:        utils.FlagTypeString,
				Shorthand:   "k",
				Description: "Private key in hex format",
				Required:    true,
			},
		},
		RunFunc: func(cmd *cobra.Command, args []string) error {
			var err error
			privateKey, err = cmd.Flags().GetString("private-key")
			if err != nil {
				return fmt.Errorf("failed to get private key flag: %w", err)
			}

			return ExecuteAuth(privateKey)
		},
	}, logger)

	utils.ExecuteCommand(cmd, logger)
}

func ExecuteAuth(privateKey string) error {
	logger := log.With().Str("component", "auth").Logger()

	if privateKey == "" {
		return fmt.Errorf("private key is required")
	}

	cfg, err := utils.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	privateKey = strings.TrimPrefix(privateKey, "0x")

	if len(privateKey) != 64 {
		return fmt.Errorf("invalid private key - must be 64 hex characters without 0x prefix")
	}

	_, err = crypto.HexToECDSA(privateKey)
	if err != nil {
		return fmt.Errorf("invalid private key format: %w", err)
	}

	if err := utils.SavePrivateKey(privateKey); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}

	utils.ResetClient()

	client, err := utils.GetClientWithPrivateKey(cfg, privateKey)
	if err != nil {
		return utils.WrapError(err, "invalid private key")
	}

	logger.Info().
		Str("address", client.Address().Hex()).
		Str("keystore", fmt.Sprintf("%s/%s", utils.KeystoreDirName, utils.KeystoreFileName)).
		Msg("Wallet authenticated successfully")

	return nil
}
