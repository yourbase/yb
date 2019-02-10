package main

import (
	"context"
	"flag"
	"github.com/johnewart/subcommands"
	"os"
	"path"
)

func main() {
	cmdr := subcommands.NewCommander(flag.CommandLine, path.Base(os.Args[0]))
	cmdr.Register(cmdr.HelpCommand(), "")
	cmdr.Register(cmdr.FlagsCommand(), "")
	cmdr.Register(cmdr.CommandsCommand(), "")
	cmdr.Register(&buildCmd{}, "")
	cmdr.Register(&execCmd{}, "")
	cmdr.Register(&workspaceCmd{}, "")
	cmdr.Register(&remoteCmd{}, "")
	cmdr.Register(&patchCmd{}, "")
	//subcommands.Register(&workspaceCreateCmd{}, "workspace")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(cmdr.Execute(ctx)))
}
