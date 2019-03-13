package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/johnewart/subcommands"
	"os"
	"strings"

	"path/filepath"
)

type runCmd struct {
	target string
}

func (*runCmd) Name() string     { return "run" }
func (*runCmd) Synopsis() string { return "Run an arbitrary command" }
func (*runCmd) Usage() string {
	return `run [-t|--target pkg] command`
}

func (p *runCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.target, "target", "t", "Target package, if not the default")
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

	if b.target != "" {
		targetPackage = b.target
	}

	instructions, err := workspace.LoadPackageManifest(targetPackage)
	if err != nil {
		fmt.Printf("Error getting package manifest for %s: %v\n", targetPackage, err)
		return subcommands.ExitFailure
	}

	fmt.Printf("Setting up dependencies...\n")
	workspace.SetupRuntimeDependencies(*instructions)

	fmt.Printf("Setting environment variables...\n")
	for _, property := range instructions.Exec.Environment {
		s := strings.Split(property, "=")
		if len(s) == 2 {
			fmt.Printf("  %s\n", s[0])
			os.Setenv(s[0], s[1])
		}
	}

	execDir := filepath.Join(workspace.Path, targetPackage)

	cmdString := strings.Join(f.Args(), " ")

	if err := ExecToStdout(cmdString, execDir); err != nil {
		fmt.Printf("Failed to run %s: %v", cmdString, err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}
