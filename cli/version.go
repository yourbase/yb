package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/johnewart/subcommands"
)

type VersionCmd struct {
	Version string
	Channel string
	Date    string
}

func (*VersionCmd) Name() string     { return "version" }
func (*VersionCmd) Synopsis() string { return "Show version info." }
func (*VersionCmd) Usage() string {
	return `version`
}

func (p *VersionCmd) SetFlags(f *flag.FlagSet) {
}

func (p *VersionCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	versionString := "Version: " + p.Version + " Channel: " + p.Channel
	if p.Date != "" {
		versionString = versionString + " Date: " + p.Date
	}
	fmt.Println(versionString)
	return subcommands.ExitSuccess
}
