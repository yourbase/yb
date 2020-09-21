package packages

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"

	"github.com/yourbase/yb/buildpacks"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/types"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"gopkg.in/yaml.v2"
)

func tracer() trace.Tracer {
	return global.Tracer("github.com/yourbase/yb/packages")
}

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

	plumbing.MkdirAsNeeded(workspaceDir)

	buildDir := "build"
	buildRoot := filepath.Join(workspaceDir, buildDir)

	if _, err := os.Stat(buildRoot); os.IsNotExist(err) {
		if err := os.Mkdir(buildRoot, 0700); err != nil {
			fmt.Printf("Unable to create build dir in workspace: %v\n", err)
		}
	}

	return buildRoot

}

func (p Package) SetupBuildDependencies(ctx context.Context, target types.BuildTarget) error {
	return buildpacks.LoadBuildPacks(ctx, target.Dependencies.Build, p.BuildRoot(), p.Path)
}

func (p Package) SetupRuntimeDependencies(ctx context.Context) error {
	deps := p.Manifest.Dependencies.Runtime
	deps = append(deps[:len(deps):len(deps)], p.Manifest.Dependencies.Build...)
	return buildpacks.LoadBuildPacks(ctx, deps, p.BuildRoot(), p.Path)
}
