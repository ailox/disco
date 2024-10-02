package cmd

import (
	"github.com/ailox/disco/discord"
	cli "github.com/jawher/mow.cli"
)

func server(cmd *cli.Cmd) {

	cmd.Spec = "[ -t ]"
	var (
		token = cmd.String(cli.StringOpt{
			Name:   "t token",
			Desc:   "Discord bot token",
			Value:  "",
			EnvVar: "DISCORD_TOKEN",
		})
	)

	cmd.Action = func() {
		logger.Info("Starting server")
		discord.Disco(*token)
	}
}
