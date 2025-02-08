package cli

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

var rootCmd = &cobra.Command{
	Use:   "parity",
	Short: "Parity Protocol CLI",
	Long:  `A decentralized computing network powered by blockchain and secure enclaves`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger.Init()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
