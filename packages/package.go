package packages

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"

	"github.com/yourbase/yb/buildpacks"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/types"
	"gopkg.in/yaml.v2"
)

type Package struct {
	Name     string
	Path     string
	Manifest types.BuildManifest
}

func LoadPackage(name string, path string) (Package, error) {
	manifest := types.BuildManifest{}
	buildYaml := filepath.Join(path, types.MANIFEST_FILE)
	if _, err := os.Stat(buildYaml); os.IsNotExist(err) {
		return Package{}, fmt.Errorf("Can't load %s: %v", types.MANIFEST_FILE, err)
	}

	buildyaml, _ := ioutil.ReadFile(buildYaml)
	err := yaml.Unmarshal([]byte(buildyaml), &manifest)
	if err != nil {
		return Package{}, fmt.Errorf("Error loading %s for %s: %v", types.MANIFEST_FILE, name, err)
	}
	err = mergeDeps(&manifest)
	if err != nil {
		return Package{}, fmt.Errorf("Error loading %s for %s: %v", types.MANIFEST_FILE, name, err)
	}

	p := Package{
		Path:     path,
		Name:     name,
		Manifest: manifest,
	}

	return p, nil
}

func (p Package) BuildRoot() string {
	// Are we a part of a workspace?
	workspaceDir, err := plumbing.FindWorkspaceRoot()

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

		h.Write([]byte(p.Path))
		workspaceHash := fmt.Sprintf("%x", h.Sum(nil))
		workspaceDir = filepath.Join(workspacesRoot, workspaceHash[0:12])
	}

	buildRoot := filepath.Join(workspaceDir, "build")
	if err := os.MkdirAll(buildRoot, 0777); err != nil {
		log.Errorf("Unable to create build dir in workspace: %v\n", err)
	}

	return buildRoot
}

func (p Package) SetupBuildDependencies(target types.BuildTarget) ([]types.CommandTimer, error) {
	return buildpacks.LoadBuildPacks(target.Dependencies.Build, p.BuildRoot(), p.Path)
}

func (p Package) SetupRuntimeDependencies() ([]types.CommandTimer, error) {
	deps := p.Manifest.Dependencies.Runtime
	deps = append(deps, p.Manifest.Dependencies.Build...)
	return buildpacks.LoadBuildPacks(deps, p.BuildRoot(), p.Path)
}
