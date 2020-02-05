package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/johnewart/subcommands"

	"github.com/yourbase/yb/plumbing/log"
)

type LsCmd struct {
	file string
}

func (*LsCmd) Name() string     { return "ls" }
func (*LsCmd) Synopsis() string { return "List the metadata - build targets, containers, etc." }
func (*LsCmd) Usage() string {
	return `ls`
}

func (b *LsCmd) SetFlags(f *flag.FlagSet) {
}

func (b *LsCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	targetPackage, err := GetTargetPackage()
	if err != nil {
		log.Errorf("%v", err)
		return subcommands.ExitFailure
	}

	m := targetPackage.Manifest

	fmt.Printf("Dependencies: %s\n", m.BuildDependenciesChecksum())
	return subcommands.ExitSuccess
}
