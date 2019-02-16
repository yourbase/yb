package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/johnewart/subcommands"
	"strings"

	"path/filepath"
)

type runCmd struct {
}

func (*runCmd) Name() string     { return "run" }
func (*runCmd) Synopsis() string { return "Run an arbitrary command" }
func (*runCmd) Usage() string {
	return `run [command]`
}

func (p *runCmd) SetFlags(f *flag.FlagSet) {
}

/*
Executing the target involves:
1. Map source into the target container
2. Run any dependent components
3. Start target
*/
func (b *runCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	workspace := LoadWorkspace()
	targetPackage := workspace.Target
	instructions, err := workspace.LoadPackageManifest(targetPackage)
	if err != nil {
		fmt.Printf("Error getting package manifest for %s: %v\n", targetPackage, err)
		return subcommands.ExitFailure
	}

	fmt.Printf("Setting up dependencies...\n")
	workspace.SetupRuntimeDependencies(*instructions)

	execDir := filepath.Join(workspace.Path, targetPackage)

	cmdString := strings.Join(f.Args(), " ")
	if err := ExecToStdout(cmdString, execDir); err != nil {
		fmt.Printf("Failed to run %s: %v", cmdString, err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}
