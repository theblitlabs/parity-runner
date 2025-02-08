package cli

import (
	"github.com/spf13/cobra"
	"github.com/theblitlabs/parity-protocol/cmd/auth"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with the network",
	Run: func(cmd *cobra.Command, args []string) {
		auth.Run()
	},
}

func init() {
	authCmd.Flags().String("private-key", "", "Private key in hex format")
	authCmd.MarkFlagRequired("private-key")
	rootCmd.AddCommand(authCmd)
}
