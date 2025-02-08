package cli

import (
	"github.com/spf13/cobra"
	"github.com/theblitlabs/parity-protocol/cmd/stake"
)

var stakeCmd = &cobra.Command{
	Use:   "stake",
	Short: "Stake tokens in the network",
	Run: func(cmd *cobra.Command, args []string) {
		stake.Run()
	},
}

func init() {
	stakeCmd.Flags().Float64("amount", 1.0, "Amount of PRTY tokens to stake")
	stakeCmd.MarkFlagRequired("amount")
	rootCmd.AddCommand(stakeCmd)
}
