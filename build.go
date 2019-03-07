package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/johnewart/subcommands"
	"gopkg.in/yaml.v2"

	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type buildCmd struct {
}

func (*buildCmd) Name() string     { return "build" }
func (*buildCmd) Synopsis() string { return "Build the workspace" }
func (*buildCmd) Usage() string {
	return `build`
}

func (p *buildCmd) SetFlags(f *flag.FlagSet) {
}

func (b *buildCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	workspace := LoadWorkspace()
	var targetPackage string

	fmt.Println(f.Args())
	if len(f.Args()) > 0 {
		targetPackage = f.Args()[0]
	} else {
		targetPackage = workspace.Target
	}

	fmt.Printf("Building target package %s...\n", targetPackage)

	targetDir := filepath.Join(workspace.Path, targetPackage)
	instructions := BuildInstructions{}
	buildYaml := filepath.Join(targetDir, "build.yml")
	if _, err := os.Stat(buildYaml); os.IsNotExist(err) {
		panic("No build.yml -- can't build!")
	}

	buildyaml, _ := ioutil.ReadFile(buildYaml)
	err := yaml.Unmarshal([]byte(buildyaml), &instructions)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	fmt.Printf("--- i:\n%v\n\n", instructions)

	// Ensure build deps are :+1:
	workspace.SetupBuildDependencies(instructions)

	fmt.Printf("Working in %s...\n", targetDir)

	// Set any environment variables as the last thing (override things we do in case people really want to do this)
	for _, envString := range instructions.Build.Environment {
		parts := strings.Split(envString, "=")
		key := parts[0]
		value := parts[1]
		fmt.Printf("Setting %s = %s\n", key, value)
		os.Setenv(key, value)
	}

	for _, cmdString := range instructions.Build.Commands {
		if err := ExecToStdout(cmdString, targetDir); err != nil {
			fmt.Printf("Failed to run %s: %v", cmdString, err)
			return subcommands.ExitFailure
		}
	}

	return subcommands.ExitSuccess

}
