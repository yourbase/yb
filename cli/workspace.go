package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnewart/subcommands"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"

	"github.com/yourbase/narwhal"
	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/workspace"
)

type WorkspaceCmd struct {
}

func (*WorkspaceCmd) Name() string     { return "workspace" }
func (*WorkspaceCmd) Synopsis() string { return "Workspace-related commands" }
func (*WorkspaceCmd) Usage() string {
	return `workspace <subcommand>`
}

func (w *WorkspaceCmd) SetFlags(f *flag.FlagSet) {}

func (w *WorkspaceCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	cmdr := subcommands.NewCommander(f, "workspace")
	cmdr.Register(&workspaceCreateCmd{}, "")
	cmdr.Register(&workspaceAddCmd{}, "")
	cmdr.Register(&workspaceTargetCmd{}, "")
	cmdr.Register(&workspaceLocationCmd{}, "")
	cmdr.Register(&workspaceListCmd{}, "")
	return (cmdr.Execute(ctx))
}

type workspaceListCmd struct{}

func (*workspaceListCmd) Name() string     { return "ls" }
func (*workspaceListCmd) Synopsis() string { return "List packages in workspace" }
func (*workspaceListCmd) Usage() string {
	return `ls`
}

func (w *workspaceListCmd) SetFlags(f *flag.FlagSet) {
}

func (w *workspaceListCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	ws, err := LoadWorkspace()

	if err != nil {
		log.Errorf("No package here, and no workspace, nothing to do!")
		return subcommands.ExitFailure
	}

	pkgs := ws.PackageList()

	fmt.Printf("Packages in workspace:\n")
	for _, pkg := range pkgs {
		fmt.Printf(" * %s\n", pkg.Name)
	}
	fmt.Println()

	fmt.Println("Containers in workspace:")
	if containers, err := ws.RunningContainers(ctx); err != nil {
		log.Warnf("Unable to get running containers: %v", err)
	} else {

		for _, c := range containers {

			running, err := narwhal.IsRunning(ctx, narwhal.DockerClient(), c.Container.Id)
			if err != nil {
				log.Warnf("Unable to determine if %s is running: %v", c.Container.Name, err)
			}

			status := "idle"
			if running {
				if address, err := narwhal.IPv4Address(ctx, narwhal.DockerClient(), c.Container.Id); err == nil {
					status = fmt.Sprintf("up @ %s", address)
				} else {
					status = "up @ unknown ip"
				}
			}

			fmt.Printf(" * %s (%s)", c.Container.Name, status)
		}
	}

	return subcommands.ExitSuccess
}

// LOCATION
type workspaceLocationCmd struct{}

func (*workspaceLocationCmd) Name() string     { return "locate" }
func (*workspaceLocationCmd) Synopsis() string { return "Location of workspace" }
func (*workspaceLocationCmd) Usage() string {
	return `locate`
}

func (w *workspaceLocationCmd) SetFlags(f *flag.FlagSet) {
}

func (w *workspaceLocationCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	ws, err := LoadWorkspace()

	if err != nil {
		log.Errorf("No package here, and no workspace, nothing to do!")
		return subcommands.ExitFailure
	}
	fmt.Println(ws.Root()) // No logging used, because this can be used by scripts
	return subcommands.ExitSuccess
}

// CREATION
type workspaceCreateCmd struct {
	name string
}

func (*workspaceCreateCmd) Name() string     { return "create" }
func (*workspaceCreateCmd) Synopsis() string { return "Create a new workspace" }
func (*workspaceCreateCmd) Usage() string {
	return `create --name <name>`
}

func (w *workspaceCreateCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&w.name, "name", "", "Workspace name")
}

func (w *workspaceCreateCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(w.name) == 0 {
		log.Errorf("No name provided!")
		return subcommands.ExitFailure
	}

	err := os.Mkdir(w.name, 0700)
	if err != nil {
		log.Errorf("Workspace already exists!")
		return subcommands.ExitFailure
	}

	configPath, _ := filepath.Abs(filepath.Join(w.name, "config.yml"))
	header := fmt.Sprintf("# Workspace config for %s", w.name)
	if err := ioutil.WriteFile(configPath, []byte(header), 0600); err != nil {
		log.Errorf("Unable to create initial config as %s: %v", configPath, err)
		return subcommands.ExitFailure
	}

	log.Infof("Created new workspace %s", w.name)
	return subcommands.ExitSuccess

}

// ADD PACKAGE
type workspaceAddCmd struct {
	Branch    string
	Tag       string
	Commit    string
	OneBranch bool
	Depth     int
}

func (*workspaceAddCmd) Name() string     { return "add" }
func (*workspaceAddCmd) Synopsis() string { return "Add a repository to this workspace" }
func (*workspaceAddCmd) Usage() string {
	return `add <org/repository>`
}

func (w *workspaceAddCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&w.Branch, "branch", "", "Use a specific branch when cloning (default is master)")
	f.StringVar(&w.Tag, "tag", "", "Check out a specific tag")
	f.StringVar(&w.Commit, "commit", "", "Check out a specific commit")
	f.BoolVar(&w.OneBranch, "only", false, "Only clone the branch specified (only useful with -branch, -tag or -commit)")
	f.IntVar(&w.Depth, "depth", 0, "Number of commits to fetch")
}

func (w *workspaceAddCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	// TODO: SSH Repositories...
	repository := f.Args()[0]
	var repositoryURL = repository
	cloneParts := strings.Split(repository, "/")
	cloneDir := cloneParts[len(cloneParts)-1]

	if strings.HasSuffix(cloneDir, ".git") {
		offset := strings.LastIndex(cloneDir, ".git")
		cloneDir = cloneDir[0:offset]
	}

	log.Infof("Cloning %s into %s...", repository, cloneDir)

	refName := "refs/heads/master"

	if w.Branch != "" {
		log.Infof("Using branch %s", w.Branch)
		refName = fmt.Sprintf("refs/heads/%s", w.Branch)
	}

	if w.Tag != "" {
		log.Infof("Using tag %s", w.Tag)
		refName = fmt.Sprintf("refs/tags/%s", w.Tag)
	}

	//TODO: warn if they specified more than one, use the most specific for now

	cloneOpts := git.CloneOptions{
		URL:           repositoryURL,
		Progress:      nil,
		ReferenceName: plumbing.ReferenceName(refName),
		SingleBranch:  w.OneBranch,
		Depth:         w.Depth,
	}
	_, err := git.PlainClone(cloneDir, false, &cloneOpts)

	if err != nil {
		log.Errorf("Error: %v", err)
		log.Warnln("Authentication required")

		// Try again with HTTP Auth
		// TODO only do this if the URL has github?
		githubtoken, exists := os.LookupEnv("YOURBASE_GITHUB_TOKEN")

		var auth http.BasicAuth
		if exists {
			log.Infof("Using GitHub token")
			auth = http.BasicAuth{Username: "yourbase", Password: githubtoken}
		} else {

			gituser, exists := os.LookupEnv("YOURBASE_GIT_USERNAME")
			if !exists {
				reader := bufio.NewReader(os.Stdin)
				fmt.Print("Username: ")
				gituser, _ = reader.ReadString('\n')
			}

			gitpassword, exists := os.LookupEnv("YOURBASE_GIT_PASSWORD")
			if !exists {
				reader := bufio.NewReader(os.Stdin)
				fmt.Print("Password: ")
				gitpassword, _ = reader.ReadString('\n')
			}

			auth = http.BasicAuth{Username: gituser, Password: gitpassword}
		}

		cloneOpts.Auth = &auth

		_, err := git.PlainClone(cloneDir, false, &cloneOpts)
		if err != nil {
			log.Errorf("Unable to clone repository, even with authentication: %v", err)
			return subcommands.ExitFailure
		}
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
	if len(f.Args()) == 0 {
		log.Errorf("Needs a target definition")
		return subcommands.ExitFailure
	}

	packageName := f.Args()[0]

	log.Infof("Setting %s as target", packageName)

	workspace, err := LoadWorkspace()
	if err != nil {
		log.Errorf("Can't load workspace: %v", err)
		return subcommands.ExitFailure
	}

	workspace.Target = packageName
	err = workspace.Save()

	if err != nil {
		log.Errorf("Unable to save target definition to the workspace: %v", err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}
