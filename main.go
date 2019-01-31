package main

import (
	"context"
	"flag"
	"os"

	"github.com/google/subcommands"
)

func main() {
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(&buildCmd{}, "")
	subcommands.Register(&printCmd{}, "print")
	//subcommands.Register(&workspaceCmd{}, "workspace")
	subcommands.Register(&workspaceCreateCmd{}, "workspace")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}
