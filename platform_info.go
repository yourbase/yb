package main

import (
	"context"
	"flag"

	"github.com/matishsiao/goInfo"
	"github.com/johnewart/subcommands"
)

type platformCmd struct {
}

func (*platformCmd) Name() string     { return "platform" }
func (*platformCmd) Synopsis() string { return "Show platform info." }
func (*platformCmd) Usage() string {
	return `platform`
}

func (p *platformCmd) SetFlags(f *flag.FlagSet) {
}

func (p *platformCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	gi := goInfo.GetInfo()
	gi.VarDump()
	return subcommands.ExitSuccess
}
