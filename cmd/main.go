package main

import (
	"os"

	"github.com/theblitlabs/parity-protocol/cmd/auth"
	"github.com/theblitlabs/parity-protocol/cmd/balance"
	"github.com/theblitlabs/parity-protocol/cmd/chain"
	"github.com/theblitlabs/parity-protocol/cmd/runner"
	"github.com/theblitlabs/parity-protocol/cmd/server"
	"github.com/theblitlabs/parity-protocol/cmd/stake"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
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
