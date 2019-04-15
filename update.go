package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/johnewart/subcommands"
	"github.com/microclusters/artificer/selfupdate"
)

type updateCmd struct {
}

func (*updateCmd) Name() string     { return "update" }
func (*updateCmd) Synopsis() string { return "Show update info." }
func (*updateCmd) Usage() string {
	return `update`
}

func (p *updateCmd) SetFlags(f *flag.FlagSet) {
}

func (p *updateCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if err := selfupdate.Update(); err != nil {
		fmt.Printf("Unable to self update: %v\n", err)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}
