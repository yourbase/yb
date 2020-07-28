package cli

import (
	"context"
	"flag"
	"github.com/johnewart/subcommands"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/workspace"
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

	ws, err := workspace.LoadWorkspace()
	if err != nil {
		log.Errorf("Error loading workspace: %v", err)
		return subcommands.ExitFailure
	}

	var pkg workspace.Package
	if len(f.Args()) > 0 {
		pkgName := f.Arg(0)
		pkg, err = ws.PackageByName(pkgName)
		if err != nil {
			log.Errorf("Unable to find package named %s: %v", pkgName, err)
			return subcommands.ExitFailure
		}
	} else {
		pkg, err = ws.TargetPackage()
		if err != nil {
			log.Errorf("Unable to find default package: %v", err)
			return subcommands.ExitFailure
		}
	}

	err = ws.ExecutePackage(pkg)
	if err != nil {
		log.Errorf("Unable to run '%s': %v", pkg.Name, err)
		return subcommands.ExitFailure
	}


	return subcommands.ExitSuccess
}
