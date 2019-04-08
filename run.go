package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/johnewart/subcommands"
	"os"
	"os/exec"
	"strings"
	//"path/filepath"
)

type runCmd struct {
	target      string
	environment string
}

func (*runCmd) Name() string     { return "run" }
func (*runCmd) Synopsis() string { return "Run an arbitrary command" }
func (*runCmd) Usage() string {
	return `run [-t pkg] [-e environment] command`
}

func (p *runCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.target, "t", "", "Target package, if not the default")
	f.StringVar(&p.environment, "e", "default", "The environment to set")
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
	workspace.SetupBuildDependencies(*instructions)

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

	execDir, _ := os.Getwd()
	//execDir := filepath.Join(workspace.Path, targetPackage)

	fmt.Printf("Running %s from %s\n", strings.Join(f.Args(), " "), execDir)
	cmdName := f.Args()[0]
	cmdArgs := f.Args()[1:]
	cmd := exec.Command(cmdName, cmdArgs...)
	cmd.Dir = execDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Run()

	return subcommands.ExitSuccess
}
