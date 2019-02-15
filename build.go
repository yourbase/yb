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
	//err := os.Chdir(targetPackage)
	//if err != nil {
	//	return subcommands.ExitFailure
	//}

	//defer os.Chdir("..")

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

	var bt BuildTool
	parts := strings.Split(instructions.Build.Tool, ":")
	tool := parts[0]

	fmt.Printf("Would use tool: %s\n", instructions.Build.Tool)
	fmt.Printf("Parts: %s\n", parts)
	switch tool {
	case "node":
		bt = NewNodeBuildTool(instructions)
	default:
		fmt.Errorf("Unknown build tool: %s", instructions.Build.Tool)
	}

	if err != nil {
		return subcommands.ExitFailure
	}

	// Do it
	bt.DoBuild()
	return subcommands.ExitSuccess

}
