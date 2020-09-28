package cli

import (
	"context"
	"flag"

	"github.com/johnewart/subcommands"
	"zombiezen.com/go/log"
)

type CheckConfigCmd struct {
	file string
}

func (*CheckConfigCmd) Name() string     { return "checkconfig" }
func (*CheckConfigCmd) Synopsis() string { return "Check the config file syntax" }
func (*CheckConfigCmd) Usage() string {
	return `checkconfig [-file FILE]
Validate the local YourBase config file, .yourbase.yml by default.
`
}

func (b *CheckConfigCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&b.file, "file", "", "YAML file to check, or else the default: .yourbase.yml")
}

func (b *CheckConfigCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	targetPackage, err := GetTargetPackageNamed(b.file)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}

	log.Infof(ctx, "Config syntax for package '%s' is OK: your package is yourbased!", targetPackage.Name)

	return subcommands.ExitSuccess
}
