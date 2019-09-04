package cli

import (
	"context"
	"flag"

	"github.com/johnewart/subcommands"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/selfupdate"
)

type UpdateCmd struct {
	Version string
	Channel string
}

func (*UpdateCmd) Name() string     { return "update" }
func (*UpdateCmd) Synopsis() string { return "Show update info." }
func (*UpdateCmd) Usage() string {
	return `update
`
}

func (p *UpdateCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.Version, "version", "", "Update / downgrade to a specific version")
	f.StringVar(&p.Channel, "channel", "stable", "Which channel to use")
}

func (p *UpdateCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if err := selfupdate.Update(p.Version, p.Channel); err != nil {
		log.Errorf("Unable to self update: %v", err)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}
