package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"github.com/johnewart/subcommands"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
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
	return `create --name <name>`
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

	configPath, _ := filepath.Abs(filepath.Join(w.name, "config.yml"))
	header := fmt.Sprintf("# Workspace config for %s", w.name)
	if err := ioutil.WriteFile(configPath, []byte(header), 0600); err != nil {
		fmt.Printf("Unable to create initial config as %s: %v\n", configPath, err)
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

	// TODO: SSH Repositories...
	repository := f.Args()[0]
	var repositoryURL = repository
	cloneParts := strings.Split(repository, "/")
	cloneDir := cloneParts[len(cloneParts)-1]

	if strings.HasSuffix(cloneDir, ".git") {
		offset := strings.LastIndex(cloneDir, ".git")
		cloneDir = cloneDir[0:offset]
	}

	fmt.Printf("Cloning %s into %s...\n", repository, cloneDir)
	cloneOpts := git.CloneOptions{
		URL:      repositoryURL,
		Progress: nil,
	}
	_, err := git.PlainClone(cloneDir, false, &cloneOpts)

	if err != nil {
		fmt.Println("Authentication required")

		// Try again with HTTP Auth
		// TODO only do this if the URL has github?
		githubtoken, exists := os.LookupEnv("YOURBASE_GITHUB_TOKEN")

		var auth http.BasicAuth
		if exists {
			fmt.Println("Using GitHub token")
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
			fmt.Printf("Unable to clone repository, even with authentication: %v\n", err)
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
	Target           string `yaml:"target"`
	Path             string
	PackageWorkspace bool
}

func (w Workspace) PackagePath(target string) string {
	if w.PackageWorkspace {
		return w.Path
	} else {
		return filepath.Join(w.Path, target)
	}
}

// Change this
func (w Workspace) Root() string {
	return w.Path
}

func (w Workspace) PackageList() []string {

	var packages []string

	// Package-workspace has only one package, empty list
	if !w.PackageWorkspace {
		globStr := filepath.Join(w.Path, "*")
		files, err := filepath.Glob(globStr)
		if err != nil {
			log.Fatal(err)
		}

		for _, f := range files {
			fi, err := os.Stat(f)
			if err != nil {
				panic(err)
			}
			if fi.IsDir() {
				pkgName := strings.Replace(f, w.Path, "", 1)
				if !strings.HasPrefix(pkgName, ".") {
					packages = append(packages, pkgName)
				}
			}
		}
	}

	return packages
}

func (w Workspace) LoadPackageManifest(packageName string) (*BuildManifest, error) {
	manifest := BuildManifest{}
	buildYaml := filepath.Join(w.PackagePath(packageName), MANIFEST_FILE)
	if _, err := os.Stat(buildYaml); os.IsNotExist(err) {
		return nil, fmt.Errorf("Can't load %s: %v", MANIFEST_FILE, err)
	}

	buildyaml, _ := ioutil.ReadFile(buildYaml)
	err := yaml.Unmarshal([]byte(buildyaml), &manifest)
	if err != nil {
		return nil, fmt.Errorf("Error loading %s for %s: %v", MANIFEST_FILE, packageName, err)
	}

	return &manifest, nil

}

func (w Workspace) SetupDependencies(dependencies []string) ([]CommandTimer, error) {
	setupTimers := make([]CommandTimer, 0)

	for _, toolSpec := range dependencies {
		var bt BuildTool
		parts := strings.Split(toolSpec, ":")
		toolType := parts[0]

		fmt.Printf("Configuring build tool: %s\n", toolSpec)

		switch toolType {
		case "r":
			bt = NewRLangBuildTool(toolSpec)
		case "heroku":
			bt = NewHerokuBuildTool(toolSpec)
		case "node":
			bt = NewNodeBuildTool(toolSpec)
		case "glide":
			bt = NewGlideBuildTool(toolSpec)
		case "android":
			bt = NewAndroidBuildTool(toolSpec)
		case "gradle":
			bt = NewGradleBuildTool(toolSpec)
		case "flutter":
			bt = NewFlutterBuildTool(toolSpec)
		case "dart":
			bt = NewDartBuildTool(toolSpec)
		case "rust":
			bt = NewRustBuildTool(toolSpec)
		case "java":
			bt = NewJavaBuildTool(toolSpec)
		case "maven":
			bt = NewMavenBuildTool(toolSpec)
		case "go":
			bt = NewGolangBuildTool(toolSpec)
		case "python":
			bt = NewPythonBuildTool(toolSpec)
		case "ruby":
			bt = NewRubyBuildTool(toolSpec)
		case "homebrew":
			bt = NewHomebrewBuildTool(toolSpec)
		default:
			fmt.Printf("Ignoring unknown build tool: %s\n", toolSpec)
		}

		// Install if needed
		startTime := time.Now()
		if err := bt.Install(); err != nil {
			return setupTimers, fmt.Errorf("Unable to install tool %s: %v", toolSpec, err)
		}
		endTime := time.Now()
		setupTimers = append(setupTimers, CommandTimer{
			Command:   fmt.Sprintf("%s [install]", toolSpec),
			StartTime: startTime,
			EndTime:   endTime,
		})

		// Setup build tool (paths, env, etc)
		startTime = time.Now()
		if err := bt.Setup(); err != nil {
			return setupTimers, fmt.Errorf("Unable to setup tool %s: %v", toolSpec, err)
		}
		endTime = time.Now()
		setupTimers = append(setupTimers, CommandTimer{
			Command:   fmt.Sprintf("%s [setup]", toolSpec),
			StartTime: startTime,
			EndTime:   endTime,
		})

	}

	return setupTimers, nil

}

func (w Workspace) SetupBuildDependencies(manifest BuildManifest) ([]CommandTimer, error) {
	return w.SetupDependencies(manifest.Dependencies.Build)
}

func (w Workspace) SetupRuntimeDependencies(manifest BuildManifest) ([]CommandTimer, error) {
	return w.SetupDependencies(manifest.Dependencies.Runtime)
}

func (w Workspace) BuildRoot() string {
	var workspaceDir string

	if w.PackageWorkspace {
		workspacesRoot, exists := os.LookupEnv("YB_WORKSPACES_ROOT")
		if !exists {
			u, err := user.Current()
			if err != nil {
				workspacesRoot = "/tmp/yourbase/workspaces"
			} else {
				workspacesRoot = fmt.Sprintf("%s/.yourbase/workspaces", u.HomeDir)
			}
		}

		h := sha256.New()

		h.Write([]byte(w.Path))
		workspaceHash := fmt.Sprintf("%x", h.Sum(nil))
		workspaceDir = filepath.Join(workspacesRoot, workspaceHash[0:12])

	} else {
		workspaceDir = w.Path
	}

	fmt.Printf("Workspace root in %s\n", workspaceDir)
	MkdirAsNeeded(workspaceDir)

	buildDir := "build"
	buildRoot := filepath.Join(workspaceDir, buildDir)

	if _, err := os.Stat(buildRoot); os.IsNotExist(err) {

		if err := os.Mkdir(buildRoot, 0700); err != nil {
			fmt.Printf("Unable to create build dir in workspace: %v\n", err)
		}
	}

	return buildRoot
}

func FindFileUpTree(filename string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	for {
		file_path := filepath.Join(wd, filename)
		if _, err := os.Stat(file_path); err == nil {
			return wd, nil
		}

		wd = filepath.Dir(wd)

		if strings.HasSuffix(wd, "/") {
			return "", fmt.Errorf("Can't find %s, ended up at the root...", filename)
		}
	}
}

func FindNearestManifestFile() (string, error) {
	return FindFileUpTree(MANIFEST_FILE)
}

/*
 * Look in the directory above the manifest file, if there's a config.yml, use that
 * otherwise we use the directory of the manifest file as the workspace root
 */
func FindWorkspaceRoot() (string, error) {
	wd, err := os.Getwd()

	if err != nil {
		panic(err)
	}

	if _, err := os.Stat(filepath.Join(wd, "config.yml")); err == nil {
		// If we're currently in the directory with the config.yml
		return wd, nil
	}

	// Look upwards to find a manifest file
	packageDir, err := FindNearestManifestFile()

	// If we find a manifest file, check the parent directory for a config.yml
	if err == nil {
		parent := filepath.Dir(packageDir)
		if _, err := os.Stat(filepath.Join(parent, "config.yml")); err == nil {
			return parent, nil
		}
	} else {
		return "", err
	}

	// No config in the parent? Use the packageDir
	return packageDir, nil
}

func LoadWorkspace() Workspace {
	workspacePath, err := FindWorkspaceRoot()

	if err != nil {
		log.Fatalf("Error getting workspace path: %v", err)
	}

	var workspace = Workspace{}
	configFile := filepath.Join(workspacePath, "config.yml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Not a multi-package workspace, make sure manifest is present
		if os.Stat(filepath.Join(workspacePath, MANIFEST_FILE)); err != nil {
			// Manifest present, single-package workspace
			workspace.PackageWorkspace = true
		}
	} else {
		configyaml, _ := ioutil.ReadFile(configFile)
		err = yaml.Unmarshal([]byte(configyaml), &workspace)

		if err != nil {
			log.Fatalf("Error loading workspace config!")
		}
	}

	workspace.Path = workspacePath
	return workspace
}

func SaveWorkspace(w Workspace) error {
	if !w.PackageWorkspace {
		d, err := yaml.Marshal(w)
		if err != nil {
			log.Fatalf("error: %v", err)
			return err
		}
		err = ioutil.WriteFile("config.yml", d, 0644)
		if err != nil {
			log.Fatalf("Unable to write config: %v", err)
			return err
		}
	}
	return nil
}
