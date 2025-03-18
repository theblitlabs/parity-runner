package cli

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/theblitlabs/parity-runner/internal/utils/cliutil"
	"github.com/theblitlabs/parity-runner/internal/utils/configutil"
	"github.com/theblitlabs/parity-runner/internal/utils/errorutil"
	"github.com/theblitlabs/parity-runner/internal/utils/keystoreutil"
	"github.com/theblitlabs/parity-runner/internal/utils/walletutil"
)

func RunAuth() {
	var privateKey string
	logger := log.With().Str("component", "auth").Logger()

	cmd := cliutil.CreateCommand(cliutil.CommandConfig{
		Use:   "auth",
		Short: "Authenticate with the network",
		Flags: map[string]cliutil.Flag{
			"private-key": {
				Type:        cliutil.FlagTypeString,
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

	cliutil.ExecuteCommand(cmd, logger)
}

func ExecuteAuth(privateKey string) error {
	logger := log.With().Str("component", "auth").Logger()

	if privateKey == "" {
		return fmt.Errorf("private key is required")
	}

	cfg, err := configutil.GetConfig()
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

	if err := keystoreutil.SavePrivateKey(privateKey); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}

	walletutil.ResetClient()

	client, err := walletutil.GetClientWithPrivateKey(cfg, privateKey)
	if err != nil {
		return errorutil.WrapError(err, "invalid private key")
	}

	logger.Info().
		Str("address", client.Address().Hex()).
		Str("keystore", fmt.Sprintf("%s/%s", keystoreutil.KeystoreDirName, keystoreutil.KeystoreFileName)).
		Msg("Wallet authenticated successfully")

	return nil
}
