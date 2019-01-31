package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/google/subcommands"
	"os"
)

type workspaceCmd struct {
}

func (*workspaceCmd) Name() string     { return "workspace" }
func (*workspaceCmd) Synopsis() string { return "Workspace-related commands" }
func (*workspaceCmd) Usage() string {
	return `workspace <subcommand>`
}

func (w *workspaceCmd) SetFlags(f *flag.FlagSet) {}

func (w *workspaceCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	return subcommands.ExitFailure
}

// CREATION
type workspaceCreateCmd struct {
	name string
}

func (*workspaceCreateCmd) Name() string     { return "createworkspace" }
func (*workspaceCreateCmd) Synopsis() string { return "Create a new workspace" }
func (*workspaceCreateCmd) Usage() string {
	return `createworkspace <directory>`
}

func (w *workspaceCreateCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&w.name, "name", "", "Workspace name")
}

func (w *workspaceCreateCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(w.name) == 0 {
		fmt.Printf("No name provided!\n")
		return subcommands.ExitFailure
	}

	err := os.Mkdir(w.name, 0700)
	if err != nil {
		fmt.Printf("Workspace already exists!\n")
		return subcommands.ExitFailure
	}

	fmt.Printf("Created new workspace %s\n", w.name)
	return subcommands.ExitSuccess

}
