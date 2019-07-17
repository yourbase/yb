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
}

func (*VersionCmd) Name() string     { return "version" }
func (*VersionCmd) Synopsis() string { return "Show version info." }
func (*VersionCmd) Usage() string {
	return `version`
}

func (p *VersionCmd) SetFlags(f *flag.FlagSet) {
}

func (p *VersionCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	versionString := fmt.Sprintf("Version: %s channel: %s", p.Version, p.Channel)
	fmt.Println(versionString)
	return subcommands.ExitSuccess
}
