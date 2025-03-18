package cliutil

import (
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// CommandConfig holds configuration for a command
type CommandConfig struct {
	// Command name and metadata
	Use     string
	Short   string
	Long    string
	Example string

	// Function to run when the command is executed
	RunFunc func(cmd *cobra.Command, args []string) error

	// Flags
	Flags map[string]Flag
}

// Flag represents a command line flag
type Flag struct {
	// Flag type and metadata
	Type        FlagType
	Shorthand   string
	Description string
	Required    bool

	// Default values for different flag types
	DefaultString  string
	DefaultInt     int
	DefaultFloat64 float64
	DefaultBool    bool
}

// FlagType defines the type of flag
type FlagType int

const (
	// FlagTypeString is a string flag
	FlagTypeString FlagType = iota
	// FlagTypeInt is an integer flag
	FlagTypeInt
	// FlagTypeFloat64 is a float64 flag
	FlagTypeFloat64
	// FlagTypeBool is a boolean flag
	FlagTypeBool
)

// CreateCommand creates a new cobra command with the given configuration
func CreateCommand(config CommandConfig, log zerolog.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:     config.Use,
		Short:   config.Short,
		Long:    config.Long,
		Example: config.Example,
		RunE: func(cmd *cobra.Command, args []string) error {
			if config.RunFunc != nil {
				return config.RunFunc(cmd, args)
			}
			return nil
		},
	}

	// Add flags
	for name, flag := range config.Flags {
		switch flag.Type {
		case FlagTypeString:
			cmd.Flags().StringP(name, flag.Shorthand, flag.DefaultString, flag.Description)
		case FlagTypeInt:
			cmd.Flags().IntP(name, flag.Shorthand, flag.DefaultInt, flag.Description)
		case FlagTypeFloat64:
			cmd.Flags().Float64P(name, flag.Shorthand, flag.DefaultFloat64, flag.Description)
		case FlagTypeBool:
			cmd.Flags().BoolP(name, flag.Shorthand, flag.DefaultBool, flag.Description)
		}

		if flag.Required {
			if err := cmd.MarkFlagRequired(name); err != nil {
				log.Error().Err(err).Str("flag", name).Msg("Failed to mark flag as required")
			}
		}
	}

	return cmd
}

// ExecuteCommand executes a cobra command and handles errors
func ExecuteCommand(cmd *cobra.Command, log zerolog.Logger) {
	if err := cmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Command execution failed")
	}
}
