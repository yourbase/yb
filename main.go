package main

import (
	"context"
	"flag"
	"github.com/johnewart/subcommands"
	"os"
	"path"
)

var (
	version string
	date    string
)

func main() {

	cmdr := subcommands.NewCommander(flag.CommandLine, path.Base(os.Args[0]))
	cmdr.Register(cmdr.HelpCommand(), "")
	cmdr.Register(cmdr.FlagsCommand(), "")
	cmdr.Register(cmdr.CommandsCommand(), "")
	cmdr.Register(&buildCmd{}, "")
	cmdr.Register(&execCmd{}, "")
	cmdr.Register(&runCmd{}, "")
	cmdr.Register(&workspaceCmd{}, "")
	cmdr.Register(&remoteCmd{}, "")
	cmdr.Register(&packageCmd{}, "")
	cmdr.Register(&patchCmd{}, "")
	cmdr.Register(&loginCmd{}, "")
	cmdr.Register(&platformCmd{}, "")
	cmdr.Register(&updateCmd{}, "")
	cmdr.Register(&versionCmd{}, "")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(cmdr.Execute(ctx)))
}
