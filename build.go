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
	capitalize bool
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
	targetPackage := workspace.Target

	fmt.Printf("Building target package %s...\n", targetPackage)

	instructions := BuildInstructions{}
	buildYaml := fmt.Sprintf("%s/build.yml", targetPackage)
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

	targetDir := filepath.Join(workspace.Path, workspace.Target)
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
