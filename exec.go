package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/johnewart/subcommands"
	"log"
	"os"
	"strings"

	"path/filepath"
)

type execCmd struct {
}

func (*execCmd) Name() string { return "exec" }
func (*execCmd) Synopsis() string {
	return "Execute a project in the workspace, defaults to target project"
}
func (*execCmd) Usage() string {
	return `exec [project]`
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

	var targetPackage string

	if len(f.Args()) > 0 {
		targetPackage = f.Args()[0]
	} else {
		targetPackage = workspace.Target
	}

	instructions, err := workspace.LoadPackageManifest(targetPackage)
	if err != nil {
		fmt.Printf("Error getting package manifest for %s: %v\n", targetPackage, err)
		return subcommands.ExitFailure
	}

	workspace.SetupRuntimeDependencies(*instructions)
	containers := instructions.Exec.Dependencies.Containers

	fmt.Printf("Starting %d dependencies...\n", len(containers))
	if len(containers) > 0 {
		sc, err := NewServiceContext(targetPackage, containers)
		if err != nil {
			fmt.Sprintf("Couldn't create service context for dependencies: %v\n", err)
		}

		if err = sc.StandUp(); err != nil {
			fmt.Sprintf("Couldn't start dependencies: %v\n", err)
		}
	}

	fmt.Printf("Setting environment variables...\n")
	for _, property := range instructions.Exec.Environment {
		s := strings.Split(property, "=")
		if len(s) == 2 {
			fmt.Printf("  %s\n", s[0])
			os.Setenv(s[0], s[1])
		}
	}

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
