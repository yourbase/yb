package workspace

import (
	"crypto/sha256"
	"fmt"
	. "github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
	. "github.com/yourbase/yb/types"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

type Package struct {
	Name     string
	path     string
	Manifest BuildManifest
}

func (p Package) Path() string {
	return p.path
}


func (p Package) BuildTargetToWriter(buildTargetName string, output io.Writer) ([]CommandTimer, error) {
	manifest := p.Manifest


	// Named target, look for that and resolve it
	_, err := manifest.ResolveBuildTargets(buildTargetName)

	if err != nil {
		log.Errorf("Could not compute build target '%s': %v", buildTargetName, err)
		log.Errorf("Valid build targets: %s", strings.Join(manifest.BuildTargetList(), ", "))
		return []CommandTimer{}, err
	}

	primaryTarget, err := manifest.BuildTarget(buildTargetName)
	if err != nil {
		log.Errorf("Couldn't get primary build target '%s' specs: %v", buildTargetName, err)
		return []CommandTimer{}, err
	}

	contextId := fmt.Sprintf("%s-build-%s", p.Name, primaryTarget.Name)

	runtimeCtx := runtime.NewRuntime(contextId, p.BuildRoot())

	return primaryTarget.Build(runtimeCtx, p.Path(), p.Manifest.Dependencies.Build)
}

func (p Package) BuildTarget(name string) ([]CommandTimer, error) {
	return p.BuildTargetToWriter(name, os.Stdout)
}

func LoadPackage(name string, path string) (Package, error) {
	manifest := BuildManifest{}
	buildYaml := filepath.Join(path, MANIFEST_FILE)
	if _, err := os.Stat(buildYaml); os.IsNotExist(err) {
		return Package{}, fmt.Errorf("Can't load %s: %v", MANIFEST_FILE, err)
	}

	buildyaml, _ := ioutil.ReadFile(buildYaml)
	err := yaml.Unmarshal([]byte(buildyaml), &manifest)
	if err != nil {
		return Package{}, fmt.Errorf("Error loading %s for %s: %v", MANIFEST_FILE, name, err)
	}

	p := Package{
		path:     path,
		Name:     name,
		Manifest: manifest,
	}

	return p, nil
}

func (p Package) BuildRoot() string {
	// Are we a part of a workspace?
	workspaceDir, err := FindWorkspaceRoot()

	if err != nil {
		// Nope, just ourselves...
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

		h.Write([]byte(p.path))
		workspaceHash := fmt.Sprintf("%x", h.Sum(nil))
		workspaceDir = filepath.Join(workspacesRoot, workspaceHash[0:12])
	}

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


func (p Package) SetupRuntimeDependencies() ([]CommandTimer, error) {
	deps := p.Manifest.Dependencies.Runtime
	deps = append(deps, p.Manifest.Dependencies.Build...)
	return []CommandTimer{}, nil
	//return LoadBuildPacks(deps, p)
}


func (p Package) ExecutionRuntime(environment string) (*runtime.Runtime, error) {
	manifest := p.Manifest
	containers := manifest.Exec.Dependencies.ContainerList()
	contextId := fmt.Sprintf("%s-exec", p.Name)

	localContainerWorkDir := filepath.Join(p.BuildRoot(), "containers")
	MkdirAsNeeded(localContainerWorkDir)

	log.Infof("Will use %s as the dependency work dir", localContainerWorkDir)

	execContainer := manifest.Exec.Container
	execContainer.Environment = manifest.Exec.EnvironmentVariables(environment)
	execContainer.Command = "/usr/bin/tail -f /dev/null"
	execContainer.Label = "exec"

	// Add package to mounts @ /workspace
	sourceMapDir := "/workspace"
	if execContainer.WorkDir != "" {
		sourceMapDir = execContainer.WorkDir
	}
	log.Infof("Will mount package %s at %s in container", p.Path(), sourceMapDir)
	mount := fmt.Sprintf("%s:%s", p.Path(), sourceMapDir)
	execContainer.Mounts = append(execContainer.Mounts, mount)
	containers = append(containers, execContainer)

	return runtime.NewRuntime(contextId, p.BuildRoot()), nil
}

func (p Package) Execute(execRuntime *runtime.Runtime) error {
	for _, cmdString := range p.Manifest.Exec.Commands {
		p := runtime.Process{
			Command:     cmdString,
			Directory:   "/workspace",
			Interactive: false,
		}

		if err := execRuntime.DefaultTarget.Run(p); err != nil {
			return fmt.Errorf("Unable to run command '%s': %v", cmdString, err)
		}
	}

	return nil
}

