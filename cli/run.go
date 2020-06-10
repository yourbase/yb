package cli

import (
	"context"
	"flag"
	"fmt"
	"github.com/yourbase/yb/workspace"
	"strings"

	"github.com/johnewart/subcommands"
	"github.com/yourbase/yb/plumbing/log"
	//"path/filepath"
)

type RunCmd struct {
	target      string
	environment string
}

func (*RunCmd) Name() string     { return "run" }
func (*RunCmd) Synopsis() string { return "Run an arbitrary command" }
func (*RunCmd) Usage() string {
	return `run [-e environment] command`
}

func (p *RunCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.environment, "e", "default", "The environment to set")
}

/*
Executing the target involves:
1. Map source into the target container
2. Run any dependent components
3. Start target
*/
func (b *RunCmd) Execute(runCtx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	if len(f.Args()) == 0 {
		fmt.Println(b.Usage())
		return subcommands.ExitFailure
	}

	ws, err := workspace.LoadWorkspace()
	if err != nil {
		log.Errorf("Error loading workspace: %v", err)
		return subcommands.ExitFailure
	}

	runtimeTarget := "default"
	argList := f.Args()
	workDir := "/workspace"

	if strings.HasPrefix(argList[0], "@") {
		runtimeTarget = argList[0][1:]
		parts := strings.Split(runtimeTarget, ":")
		if len(parts) == 2 {
			workDir = parts[1]
			runtimeTarget = parts[0]
		}
		argList = argList[1:]
	}

	cmdString := strings.Join(argList, " ")
	if runErr := ws.RunInTarget(runCtx, cmdString, workDir, runtimeTarget); runErr != nil {
		log.Errorf("Unable to run command: %v", runErr)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}
