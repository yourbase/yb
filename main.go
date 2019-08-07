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
	date    string
)

func main() {
	SetupOutput()

	cmdr := subcommands.NewCommander(flag.CommandLine, path.Base(os.Args[0]))
	cmdr.Register(cmdr.HelpCommand(), "")
	cmdr.Register(cmdr.FlagsCommand(), "")
	cmdr.Register(cmdr.CommandsCommand(), "")
	cmdr.Register(&BuildCmd{}, "")
	cmdr.Register(&CheckConfigCmd{}, "")
	cmdr.Register(&ExecCmd{}, "")
	cmdr.Register(&RunCmd{}, "")
	cmdr.Register(&WorkspaceCmd{}, "")
	cmdr.Register(&RemoteCmd{}, "")
	cmdr.Register(&PackageCmd{}, "")
	cmdr.Register(&LoginCmd{}, "")
	cmdr.Register(&PlatformCmd{}, "")
	cmdr.Register(&UpdateCmd{}, "")
	cmdr.Register(&VersionCmd{Version: version}, "")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(cmdr.Execute(ctx)))
}
