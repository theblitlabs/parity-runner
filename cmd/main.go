package main

import (
	"os"

	"github.com/virajbhartiya/parity-protocol/cmd/auth"
	server "github.com/virajbhartiya/parity-protocol/cmd/server"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
)

func main() {
	logger.Init()
	log := logger.Get()

	// Check command
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "auth":
			auth.Run()
			return
		case "server":
			server.Run()
			return
		default:
			log.Error().Msg("Unknown command. Use 'auth' or 'server'")
		}
	}

	log.Error().Msg("No command specified. Use 'parity auth' or 'parity server'")
}
