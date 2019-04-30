package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/johnewart/subcommands"
	"github.com/yourbase/yb/selfupdate"
)

type UpdateCmd struct {
}

func (*UpdateCmd) Name() string     { return "update" }
func (*UpdateCmd) Synopsis() string { return "Show update info." }
func (*UpdateCmd) Usage() string {
	return `update`
}

func (p *UpdateCmd) SetFlags(f *flag.FlagSet) {
}

func (p *UpdateCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if err := selfupdate.Update(); err != nil {
		fmt.Printf("Unable to self update: %v\n", err)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}
