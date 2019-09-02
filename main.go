package main

import (
	"context"
	"flag"
	"github.com/johnewart/subcommands"
	"os"
	"path"

	. "github.com/yourbase/yb/cli"
	"github.com/yourbase/yb/plumbing/log"
)

var (
	version     string = "DEVELOPMENT"
	date        string
	noPrettyOut bool
)

func main() {
	log.Formatter.NoPrettyOut = noPrettyOut

	cmdr := subcommands.NewCommander(flag.CommandLine, path.Base(os.Args[0]))
	cmdr.Register(cmdr.HelpCommand(), "")
	cmdr.Register(cmdr.FlagsCommand(), "")
	cmdr.Register(cmdr.CommandsCommand(), "")
	cmdr.Register(&BuildCmd{}, "")
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
	cmdr.Register(&VersionCmd{Version: version}, "")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(cmdr.Execute(ctx)))
}
