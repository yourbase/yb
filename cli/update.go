package cli

import (
	"context"
	"flag"

	"github.com/johnewart/subcommands"
	"github.com/yourbase/yb/selfupdate"
	"zombiezen.com/go/log"
)

type UpdateCmd struct {
	Version string
	Channel string
}

func (*UpdateCmd) Name() string     { return "update" }
func (*UpdateCmd) Synopsis() string { return "Self-update to the latest yb" }
func (*UpdateCmd) Usage() string {
	return `Usage: update [OPTIONS]
Update yb to the latest verison, or a specific version.
`
}

func (p *UpdateCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.Version, "version", "", "Update / downgrade to a specific version")
	f.StringVar(&p.Channel, "channel", "stable", "Which channel to use")
}

func (p *UpdateCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if err := selfupdate.Update(p.Version, p.Channel); err != nil {
		log.Errorf(ctx, "Unable to self update: %v", err)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}
