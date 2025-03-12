package main

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-protocol/cmd/cli"
)

var logMode string

var rootCmd = &cobra.Command{
	Use:   "parity-runner",
	Short: "Parity Runner",
	Long:  `A decentralized computing network powered by blockchain and secure enclaves`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		switch logMode {
		case "debug", "pretty", "info", "prod", "test":
			gologger.InitWithMode(gologger.LogMode(logMode))
		default:
			gologger.InitWithMode(gologger.LogModePretty)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		cli.RunRunner()
	},
}

func main() {
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(stakeCmd)
	rootCmd.AddCommand(runnerCmd)
	rootCmd.AddCommand(balanceCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with the network",
	Run: func(cmd *cobra.Command, args []string) {
		cli.RunAuth()
	},
}

var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Check token balances and stake status",
	Run: func(cmd *cobra.Command, args []string) {
		cli.RunBalance()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&logMode, "log", "pretty", "Log mode: debug, pretty, info, prod, test")

	authCmd.Flags().String("private-key", "", "Private key in hex format")
	if err := authCmd.MarkFlagRequired("private-key"); err != nil {
		log.Error().Err(err).Msg("Failed to mark private-key flag as required")
	}

	stakeCmd.Flags().Float64("amount", 1.0, "Amount of PRTY tokens to stake")
	if err := stakeCmd.MarkFlagRequired("amount"); err != nil {
		log.Error().Err(err).Msg("Failed to mark amount flag as required")
	}

}

var runnerCmd = &cobra.Command{
	Use:   "runner",
	Short: "Start the task runner",
	Run: func(cmd *cobra.Command, args []string) {
		cli.RunRunner()
	},
}

var stakeCmd = &cobra.Command{
	Use:   "stake",
	Short: "Stake tokens in the network",
	Run: func(cmd *cobra.Command, args []string) {
		cli.RunStake()
	},
}
