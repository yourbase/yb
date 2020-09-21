package main

import (
	"context"
	"flag"
	"os"
	"path"

	"github.com/johnewart/subcommands"
	"github.com/yourbase/yb/cli"
)

var (
	version   string = "DEVELOPMENT"
	channel   string = "development"
	date      string
	commitSHA string
)

func main() {
	cmdr := subcommands.NewCommander(flag.CommandLine, path.Base(os.Args[0]))
	cmdr.Register(cmdr.HelpCommand(), "")
	cmdr.Register(cmdr.FlagsCommand(), "")
	cmdr.Register(cmdr.CommandsCommand(), "")
	cmdr.Register(&cli.BuildCmd{Version: version, Channel: channel, CommitSHA: commitSHA}, "")
	cmdr.Register(&cli.CheckConfigCmd{}, "")
	cmdr.Register(&cli.ConfigCmd{}, "")
	cmdr.Register(&cli.ExecCmd{}, "")
	cmdr.Register(&cli.LoginCmd{}, "")
	cmdr.Register(&cli.PackageCmd{}, "")
	cmdr.Register(&cli.PlatformCmd{}, "")
	cmdr.Register(&cli.RemoteCmd{}, "")
	cmdr.Register(&cli.RunCmd{}, "")
	cmdr.Register(&cli.UpdateCmd{}, "")
	cmdr.Register(&cli.WorkspaceCmd{}, "")
	cmdr.Register(&cli.VersionCmd{Version: version, Channel: channel, Date: date, CommitSHA: commitSHA}, "")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(cmdr.Execute(ctx)))
}
