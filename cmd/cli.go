package cmd

import (
	"log"
	"strings"

	cli "github.com/jawher/mow.cli"
	"go.uber.org/zap"
)

// logger is the main logger instance for structured log output.
var logger *zap.Logger

func Run(args []string) {
	binPath := strings.Split(args[0], "/")
	binName := binPath[len(binPath)-1]
	app := cli.App(binName, "Discord music bot")

	app.Before = func() {
		// Initialize Logger to be available for all subcommands
		var err error
		logger, err = zap.NewProduction()
		if err != nil {
			log.Fatal("Unable to initialize logger instance")
		}
	}

	app.Command("run", "run bot server", server)
	app.Run(args)
}
