package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/johnewart/subcommands"
)

type versionCmd struct {
}

func (*versionCmd) Name() string     { return "version" }
func (*versionCmd) Synopsis() string { return "Show version info." }
func (*versionCmd) Usage() string {
	return `version`
}

func (p *versionCmd) SetFlags(f *flag.FlagSet) {
}

func (p *versionCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	fmt.Println(version)
	return subcommands.ExitSuccess
}
