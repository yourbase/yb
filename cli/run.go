package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/johnewart/subcommands"
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
		fmt.Printf("Can't get target package: %v\n", err)
		return subcommands.ExitFailure
	}

	instructions := targetPackage.Manifest

	fmt.Printf("Setting up dependencies...\n")
	targetPackage.SetupRuntimeDependencies()

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
