package packages

import (
	"crypto/sha256"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"

	. "github.com/yourbase/yb/buildpacks"
	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
)

type Package struct {
	Name     string
	Path     string
	Manifest BuildManifest
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
		Path:     path,
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

		h.Write([]byte(p.Path))
		workspaceHash := fmt.Sprintf("%x", h.Sum(nil))
		workspaceDir = filepath.Join(workspacesRoot, workspaceHash[0:12])
	}

	MkdirAsNeeded(workspaceDir)

	buildDir := "build"
	buildRoot := filepath.Join(workspaceDir, buildDir)

	fmt.Printf("Package build root in %s\n", buildRoot)

	if _, err := os.Stat(buildRoot); os.IsNotExist(err) {

		if err := os.Mkdir(buildRoot, 0700); err != nil {
			fmt.Printf("Unable to create build dir in workspace: %v\n", err)
		}
	}

	return buildRoot

}

func (p Package) SetupBuildDependencies() ([]CommandTimer, error) {
	return LoadBuildPacks(p.Manifest.Dependencies.Build, p.BuildRoot(), p.Path)
}

func (p Package) SetupRuntimeDependencies() ([]CommandTimer, error) {
	return LoadBuildPacks(p.Manifest.Dependencies.Runtime, p.BuildRoot(), p.Path)
}
