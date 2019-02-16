package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/johnewart/subcommands"
	"log"

	"path/filepath"
)

type execCmd struct {
	capitalize bool
}

func (*execCmd) Name() string     { return "exec" }
func (*execCmd) Synopsis() string { return "Execute the target project in the workspace" }
func (*execCmd) Usage() string {
	return `exec`
}

func (p *execCmd) SetFlags(f *flag.FlagSet) {
}

/*
Executing the target involves:
1. Map source into the target container
2. Run any dependent components
3. Start target
*/
func (b *execCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	workspace := LoadWorkspace()
	targetPackage := workspace.Target
	instructions, err := workspace.LoadPackageManifest(targetPackage)
	if err != nil {
		fmt.Printf("Error getting package manifest for %s: %v\n", targetPackage, err)
		return subcommands.ExitFailure
	}

	workspace.SetupRuntimeDependencies(*instructions)

	log.Printf("Execing target package %s...\n", targetPackage)
	execDir := filepath.Join(workspace.Path, targetPackage)

	for _, cmdString := range instructions.Exec.Commands {
		if err := ExecToStdout(cmdString, execDir); err != nil {
			fmt.Printf("Failed to run %s: %v", cmdString, err)
			return subcommands.ExitFailure
		}
	}

	return subcommands.ExitSuccess
}
