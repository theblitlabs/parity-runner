package cli

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

var (
	// Configuration flags
	logMode string
)

var rootCmd = &cobra.Command{
	Use:   "parity",
	Short: "Parity Protocol CLI",
	Long:  `A decentralized computing network powered by blockchain and secure enclaves`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize logger with the selected mode
		switch logMode {
		case "debug", "pretty", "info", "prod", "test":
			logger.InitWithMode(logger.LogMode(logMode))
		default:
			// Default to pretty logging if invalid mode provided
			logger.InitWithMode(logger.LogModePretty)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Register subcommands
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(stakeCmd)
	rootCmd.AddCommand(runnerCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(chainCmd)
	rootCmd.AddCommand(balanceCmd)
	rootCmd.AddCommand(migrateCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with the network",
	Run: func(cmd *cobra.Command, args []string) {
		RunAuth()
	},
}

var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Check token balances and stake status",
	Run: func(cmd *cobra.Command, args []string) {
		RunBalance()
	},
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&logMode, "log", "pretty", "Log mode: debug, pretty, info, prod, test")

	// Auth command flags
	authCmd.Flags().String("private-key", "", "Private key in hex format")
	if err := authCmd.MarkFlagRequired("private-key"); err != nil {
		log.Error().Err(err).Msg("Failed to mark private-key flag as required")
	}

	// Stake command flags
	stakeCmd.Flags().Float64("amount", 1.0, "Amount of PRTY tokens to stake")
	if err := stakeCmd.MarkFlagRequired("amount"); err != nil {
		log.Error().Err(err).Msg("Failed to mark amount flag as required")
	}

	// Migrate command flags
	migrateCmd.Flags().Bool("down", false, "Rollback migrations")
}

var chainCmd = &cobra.Command{
	Use:   "chain",
	Short: "Start the chain proxy server",
	Run: func(cmd *cobra.Command, args []string) {
		RunChain()
	},
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Run: func(cmd *cobra.Command, args []string) {
		down, _ := cmd.Flags().GetBool("down")
		RunMigrate(down)
	},
}

var runnerCmd = &cobra.Command{
	Use:   "runner",
	Short: "Start the task runner",
	Run: func(cmd *cobra.Command, args []string) {
		RunRunner()
	},
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the parity server",
	Run: func(cmd *cobra.Command, args []string) {
		RunServer()
	},
}

var stakeCmd = &cobra.Command{
	Use:   "stake",
	Short: "Stake tokens in the network",
	Run: func(cmd *cobra.Command, args []string) {
		RunStake()
	},
}
