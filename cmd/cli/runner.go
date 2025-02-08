package cli

import (
	"github.com/spf13/cobra"
	"github.com/theblitlabs/parity-protocol/cmd/runner"
)

var runnerCmd = &cobra.Command{
	Use:   "runner",
	Short: "Start the task runner",
	Run: func(cmd *cobra.Command, args []string) {
		runner.Run()
	},
}

func init() {
	rootCmd.AddCommand(runnerCmd)
}
