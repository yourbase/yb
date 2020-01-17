package cli

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/johnewart/subcommands"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
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
func (b *RunCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	if len(f.Args()) == 0 {
		fmt.Println(b.Usage())
		return subcommands.ExitFailure
	}

	targetPackage, err := GetTargetPackage()
	if err != nil {
		log.Errorf("%v", err)
		return subcommands.ExitFailure
	}

	manifest := targetPackage.Manifest
	containers := manifest.Exec.Dependencies.ContainerList()
	contextId := fmt.Sprintf("%s-exec", targetPackage.Name)

	opts := runtime.ContainerRuntimeOpts{
		Identifier:           contextId,
		ContainerDefinitions: containers,
	}

	runtimeEnv, err := runtime.NewContainerRuntime(opts)
	if err != nil {
		log.Errorf("%v", err)
		return subcommands.ExitFailure
	}

	runtimeTarget := "exec"
	argList := f.Args()

	if strings.HasPrefix(argList[0], "@") {
		runtimeTarget = argList[0][1:]
		argList = argList[1:]
	}

	cmdString := strings.Join(argList, " ")

	log.Infof("Running %s in %s", cmdString, runtimeTarget)

	p := runtime.Process{Command: cmdString, Interactive: true, Directory: "/"}

	if err := runtimeEnv.Run(p, runtimeTarget); err != nil {
		log.Errorf("%v", err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}
