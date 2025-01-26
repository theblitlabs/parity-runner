package main

import (
	"os"

	"github.com/virajbhartiya/parity-protocol/cmd/auth"
	"github.com/virajbhartiya/parity-protocol/cmd/balance"
	"github.com/virajbhartiya/parity-protocol/cmd/chain"
	"github.com/virajbhartiya/parity-protocol/cmd/runner"
	"github.com/virajbhartiya/parity-protocol/cmd/server"
	"github.com/virajbhartiya/parity-protocol/cmd/stake"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
)

func main() {
	logger.Init()
	log := logger.Get()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "auth":
			auth.Run()
			return
		case "server":
			server.Run()
			return
		case "runner":
			runner.Run()
			return
		case "chain":
			chain.Run()
			return
		case "stake":
			stake.Run()
			return
		case "balance":
			balance.Run()
			return
		default:
			log.Error().Msg("Unknown command. Use 'auth', 'server', 'runner', 'chain', 'stake' or 'balance'")
		}
	}

	log.Error().Msg("No command specified. Use 'parity auth|server|runner|chain|stake'")
}
