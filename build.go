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
	if PathExists(MANIFEST_FILE) {
		currentPath, _ := filepath.Abs(".")
		_, file := filepath.Split(currentPath)
		targetPackage = file
	} else {
		if len(f.Args()) > 0 {
			targetPackage = f.Args()[0]
		} else {
			targetPackage = workspace.Target
		}
	}

	fmt.Printf("Building target package %s...\n", targetPackage)

	targetDir := filepath.Join(workspace.Path, targetPackage)
	instructions := BuildInstructions{}
	buildYaml := filepath.Join(targetDir, MANIFEST_FILE)
	if _, err := os.Stat(buildYaml); os.IsNotExist(err) {
		fmt.Printf("No %s -- can't build!\n", MANIFEST_FILE)
	}

	buildyaml, _ := ioutil.ReadFile(buildYaml)
	err := yaml.Unmarshal([]byte(buildyaml), &instructions)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	fmt.Printf("Working in %s...\n", targetDir)

	// Set any environment variables as the last thing (override things we do in case people really want to do this)
	for _, envString := range instructions.Build.Environment {
		parts := strings.Split(envString, "=")
		key := parts[0]
		value := parts[1]
		value = strings.Replace(value, "{PKGDIR}", targetDir, -1)
		fmt.Printf("Setting %s = %s\n", key, value)
		os.Setenv(key, value)
	}

	if instructions.Build.Container.Image != "" {
		// Perform build inside a container
		image := instructions.Build.Container.Image
		fmt.Printf("Invoking build in a container: %s\n", image)

		buildOpts := BuildContainerOpts{
			ContainerOpts: instructions.Build.Container,
			PackageName:   targetPackage,
			Workspace:     workspace,
		}

		var buildContainer BuildContainer

		if existing := FindContainer(buildOpts); existing != nil {
			fmt.Printf("Found existing container %s, removing...\n", existing.Id)
			if err = RemoveContainerById(existing.Id); err != nil {
				fmt.Printf("Unable to remove existing container: %v\n", err)
			}
		}

		buildContainer, err = NewContainer(buildOpts)
		if err != nil {
			fmt.Printf("Error creating build container: %v\n", err)
			return subcommands.ExitFailure
		}

		if err := buildContainer.Start(); err != nil {
			fmt.Printf("Unable to start container %s: %v", buildContainer.Id, err)
			return subcommands.ExitFailure
		}

		fmt.Printf("Building in container: %s\n", buildContainer.Id)

		for _, cmdString := range instructions.Build.Commands {
			fmt.Printf("Would run %s in the container\n", cmdString)
			if err := buildContainer.ExecToStdout(cmdString); err != nil {
				fmt.Printf("Failed to run %s: %v", cmdString, err)
				return subcommands.ExitFailure
			}
		}

	} else {
		// Ensure build deps are :+1:
		workspace.SetupBuildDependencies(instructions)
		for _, cmdString := range instructions.Build.Commands {
			if strings.HasPrefix(cmdString, "cd ") {
				parts := strings.SplitN(cmdString, " ", 2)
				dir := filepath.Join(targetDir, parts[1])
				//fmt.Printf("Chdir to %s\n", dir)
				//os.Chdir(dir)
				targetDir = dir
			} else {
				if err := ExecToStdout(cmdString, targetDir); err != nil {
					fmt.Printf("Failed to run %s: %v", cmdString, err)
					return subcommands.ExitFailure
				}
			}

		}

	}

	return subcommands.ExitSuccess

}
