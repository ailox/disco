package cmd

import (
	cli "github.com/jawher/mow.cli"
)

func server(cmd *cli.Cmd) {
	cmd.Action = func() {
		logger.Info("Starting server")
	}
}
