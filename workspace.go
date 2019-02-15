package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/johnewart/subcommands"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type workspaceCmd struct {
}

func (*workspaceCmd) Name() string     { return "workspace" }
func (*workspaceCmd) Synopsis() string { return "Workspace-related commands" }
func (*workspaceCmd) Usage() string {
	return `workspace <subcommand>`
}

func (w *workspaceCmd) SetFlags(f *flag.FlagSet) {}

func (w *workspaceCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	cmdr := subcommands.NewCommander(f, "workspace")
	cmdr.Register(&workspaceCreateCmd{}, "")
	cmdr.Register(&workspaceAddCmd{}, "")
	cmdr.Register(&workspaceTargetCmd{}, "")
	return (cmdr.Execute(ctx))
	//return subcommands.ExitFailure
}

// CREATION
type workspaceCreateCmd struct {
	name string
}

func (*workspaceCreateCmd) Name() string     { return "create" }
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

// ADD PACKAGE
type workspaceAddCmd struct {
}

func (*workspaceAddCmd) Name() string     { return "add" }
func (*workspaceAddCmd) Synopsis() string { return "Add a repository to this workspace" }
func (*workspaceAddCmd) Usage() string {
	return `add <org/repository>`
}

func (w *workspaceAddCmd) SetFlags(f *flag.FlagSet) {}

func (w *workspaceAddCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	repository := f.Args()[0]
	repositoryURL := fmt.Sprintf("https://github.com/%s.git", repository)
	cloneParts := strings.Split(repository, "/")
	cloneDir := cloneParts[len(cloneParts)-1]

	fmt.Printf("Cloning %s into %s...\n", repository, cloneDir)

	_, err := git.PlainClone(cloneDir, false, &git.CloneOptions{
		URL:      repositoryURL,
		Progress: os.Stdout,
	})

	if err != nil {
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}

// SET TARGET PACKAGE
type workspaceTargetCmd struct {
}

func (*workspaceTargetCmd) Name() string     { return "target" }
func (*workspaceTargetCmd) Synopsis() string { return "Set target package" }
func (*workspaceTargetCmd) Usage() string {
	return `target <package>`
}

func (w *workspaceTargetCmd) SetFlags(f *flag.FlagSet) {}

func (w *workspaceTargetCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	packageName := f.Args()[0]

	fmt.Printf("Setting %s as target\n", packageName)

	workspace := LoadWorkspace()
	workspace.Target = packageName
	err := SaveWorkspace(workspace)

	if err != nil {
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}

// UTILITY FUNCTIONS

type Workspace struct {
	Target string `yaml:"target"`
	Path   string
}

func FindWorkspaceRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	for {
		config_path := filepath.Join(wd, "config.yml")
		fmt.Printf("Checking for %s...\n", config_path)
		if _, err := os.Stat(config_path); err == nil {
			fmt.Printf("Found!\n")
			return wd, nil
		}
		wd = filepath.Dir(wd)
		fmt.Printf("Checking %s\n", wd)
		if strings.HasSuffix(wd, "/") {
			return "", fmt.Errorf("Can't find workspace, ended up at the root...")
		}
	}
}

func LoadWorkspace() Workspace {
	workspacePath, err := FindWorkspaceRoot()

	if err != nil {
		log.Fatalf("Error getting workspace path: %v", err)
	}
	fmt.Printf("Loading workspace from %s...\n", workspacePath)

	configyaml, _ := ioutil.ReadFile("config.yml")
	var workspace = Workspace{}
	err = yaml.Unmarshal([]byte(configyaml), &workspace)

	if err != nil {
		log.Fatalf("Error loading workspace config!")
	}

	workspace.Path = workspacePath

	return workspace
}

func SaveWorkspace(w Workspace) error {
	d, err := yaml.Marshal(w)
	if err != nil {
		log.Fatalf("error: %v", err)
		return err
	}
	fmt.Printf("--- t dump:\n%s\n\n", string(d))
	err = ioutil.WriteFile("config.yml", d, 0644)
	if err != nil {
		log.Fatalf("Unable to write config: %v", err)
		return err
	}
	return nil
}
