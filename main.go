package main

import (
	"context"
	"flag"
	"github.com/johnewart/subcommands"
	"os"
	"path"

	. "github.com/yourbase/yb/cli"
)

var (
	version string = "DEVELOPMENT"
	channel string = "development"
	date    string
)

func main() {
	cmdr := subcommands.NewCommander(flag.CommandLine, path.Base(os.Args[0]))
	cmdr.Register(cmdr.HelpCommand(), "")
	cmdr.Register(cmdr.FlagsCommand(), "")
	cmdr.Register(cmdr.CommandsCommand(), "")
	cmdr.Register(&BuildCmd{Version: version, Channel: channel}, "")
	cmdr.Register(&CheckConfigCmd{}, "")
	cmdr.Register(&ConfigCmd{}, "")
	cmdr.Register(&ExecCmd{}, "")
	cmdr.Register(&LoginCmd{}, "")
	cmdr.Register(&PackageCmd{}, "")
	cmdr.Register(&PlatformCmd{}, "")
	cmdr.Register(&RemoteCmd{}, "")
	cmdr.Register(&RunCmd{}, "")
	cmdr.Register(&UpdateCmd{}, "")
	cmdr.Register(&WorkspaceCmd{}, "")
	cmdr.Register(&VersionCmd{Version: version, Channel: channel}, "")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(cmdr.Execute(ctx)))
}
