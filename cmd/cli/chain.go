package cli

import (
	"github.com/spf13/cobra"
	"github.com/theblitlabs/parity-protocol/cmd/chain"
)

var chainCmd = &cobra.Command{
	Use:   "chain",
	Short: "Start the chain proxy server",
	Run: func(cmd *cobra.Command, args []string) {
		chain.Run()
	},
}

func init() {
	rootCmd.AddCommand(chainCmd)
}
