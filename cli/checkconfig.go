package cli

import (
	"context"
	"flag"

	"github.com/johnewart/subcommands"

	"github.com/yourbase/yb/plumbing/log"
)

type CheckConfigCmd struct {
}

func (*CheckConfigCmd) Name() string     { return "checkconfig" }
func (*CheckConfigCmd) Synopsis() string { return "Check the config file syntax" }
func (*CheckConfigCmd) Usage() string {
	return `checkconfig`
}

func (b *CheckConfigCmd) SetFlags(f *flag.FlagSet) {
}

func (b *CheckConfigCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	targetPackage, err := GetTargetPackage()
	if err != nil {
		log.Errorf("%v", err)
		return subcommands.ExitFailure
	}

	log.Infof("Config syntax for package '%s' is OK: your package is yourbased!", targetPackage.Name)

	return subcommands.ExitSuccess
}
