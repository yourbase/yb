package cli

import (
	"context"
	"flag"

	"github.com/johnewart/subcommands"

	"github.com/yourbase/yb/plumbing/log"
)

type ExecCmd struct {
	environment string
}

func (*ExecCmd) Name() string { return "exec" }
func (*ExecCmd) Synopsis() string {
	return "Execute a project in the workspace, defaults to target project"
}
func (*ExecCmd) Usage() string {
	return `exec [project]`
}

func (p *ExecCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.environment, "e", "", "Environment to run as")
}

/*
Executing the target involves:
1. Map source into the target container
2. Run any dependent components
3. Start target
*/
func (b *ExecCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	targetPackage, err := GetTargetPackage()
	if err != nil {
		log.Errorf("%v", err)
		return subcommands.ExitFailure
	}

	log.ActiveSection("Dependencies")
	if _, err := targetPackage.SetupRuntimeDependencies(); err != nil {
		log.Infof("Couldn't configure dependencies: %v\n", err)
		return subcommands.ExitFailure
	}

	execRuntime, err := targetPackage.ExecutionRuntime(b.environment)

	log.Infof("Executing package '%s'...\n", targetPackage.Name)

	err = targetPackage.Execute(execRuntime)
	if err != nil {
		log.Errorf("Unable to run command: %v", err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}
