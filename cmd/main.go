package main

import (
	"os"

	"github.com/virajbhartiya/parity-protocol/cmd/auth"
	daemon "github.com/virajbhartiya/parity-protocol/cmd/server"
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
		case "daemon":
			daemon.Run()
			return
		default:
			log.Error().Msg("Unknown command. Use 'auth' or 'daemon'")
		}
	}

	log.Error().Msg("No command specified. Use 'parity auth' or 'parity daemon'")
}
