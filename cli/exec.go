package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnewart/subcommands"

	. "github.com/yourbase/yb/packages"
	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
	. "github.com/yourbase/yb/workspace"
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

	var targetPackage Package

	if PathExists(MANIFEST_FILE) {
		currentPath, _ := filepath.Abs(".")
		_, packageName := filepath.Split(currentPath)
		pkg, err := LoadPackage(packageName, currentPath)
		if err != nil {
			fmt.Printf("Can't load package '%s': %v\n", packageName, err)
			return subcommands.ExitFailure
		}
		targetPackage = pkg
	} else {
		workspace, err := LoadWorkspace()
		if err != nil {
			fmt.Printf("Can't load workspace: %v\n", err)
			return subcommands.ExitFailure
		}
		pkg, err := workspace.TargetPackage()
		if err != nil {
			fmt.Printf("Can't determine target package: %v\n", err)
			return subcommands.ExitFailure
		}

		targetPackage = pkg
	}

	if _, err := targetPackage.SetupRuntimeDependencies(); err != nil {
		fmt.Printf("Couldn't configure dependencies: %v\n", err)
		return subcommands.ExitFailure
	}

	instructions := targetPackage.Manifest
	containers := instructions.Exec.Dependencies.Containers

	fmt.Printf("Starting %d dependencies...\n", len(containers))
	contextId := targetPackage.Name
	if len(containers) > 0 {
		sc, err := NewServiceContextWithId(contextId, targetPackage, containers)
		if err != nil {
			fmt.Sprintf("Couldn't create service context for dependencies: %v\n", err)
		}

		if err = sc.StandUp(); err != nil {
			fmt.Printf("Couldn't start dependencies: %v\n", err)
			return subcommands.ExitFailure
		}
	}

	fmt.Printf("Setting environment variables...\n")
	for _, property := range instructions.Exec.Environment["default"] {
		s := strings.Split(property, "=")
		if len(s) == 2 {
			fmt.Printf("  %s\n", s[0])
			os.Setenv(s[0], s[1])
		}
	}

	if b.environment != "default" {
		for _, property := range instructions.Exec.Environment[b.environment] {
			s := strings.Split(property, "=")
			if len(s) == 2 {
				fmt.Printf("  %s\n", s[0])
				os.Setenv(s[0], s[1])
			}
		}
	}

	fmt.Printf("Execing target package %s...\n", targetPackage.Name)
	execDir := targetPackage.Path

	for _, logFile := range instructions.Exec.LogFiles {
		fmt.Printf("Will tail %s...\n", logFile)
	}

	for _, cmdString := range instructions.Exec.Commands {
		if err := ExecToStdout(cmdString, execDir); err != nil {
			fmt.Printf("Failed to run %s: %v", cmdString, err)
			return subcommands.ExitFailure
		}
	}

	return subcommands.ExitSuccess
}
