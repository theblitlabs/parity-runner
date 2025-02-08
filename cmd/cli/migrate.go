package cli

import (
	"github.com/spf13/cobra"
	"github.com/theblitlabs/parity-protocol/cmd/migrate"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Run: func(cmd *cobra.Command, args []string) {
		down, _ := cmd.Flags().GetBool("down")
		migrate.Run(down)
	},
}

func init() {
	migrateCmd.Flags().Bool("down", false, "Rollback migrations")
	rootCmd.AddCommand(migrateCmd)
}
