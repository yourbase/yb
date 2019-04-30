package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/johnewart/subcommands"
)

type VersionCmd struct {
	Version string
}

func (*VersionCmd) Name() string     { return "version" }
func (*VersionCmd) Synopsis() string { return "Show version info." }
func (*VersionCmd) Usage() string {
	return `version`
}

func (p *VersionCmd) SetFlags(f *flag.FlagSet) {
}

func (p *VersionCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	fmt.Println(p.Version)
	return subcommands.ExitSuccess
}
