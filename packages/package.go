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
	"zombiezen.com/go/log"
)

func tracer() trace.Tracer {
	return global.Tracer("github.com/yourbase/yb/packages")
}

type Package struct {
	Name     string
	Path     string
	Manifest types.BuildManifest
}

func LoadPackage(name string, path string) (*Package, error) {
	manifest := types.BuildManifest{}
	buildYaml := filepath.Join(path, types.MANIFEST_FILE)
	if _, err := os.Stat(buildYaml); os.IsNotExist(err) {
		return nil, fmt.Errorf("Can't load %s: %v", types.MANIFEST_FILE, err)
	}

	buildyaml, _ := ioutil.ReadFile(buildYaml)
	err := yaml.Unmarshal([]byte(buildyaml), &manifest)
	if err != nil {
		return nil, fmt.Errorf("Error loading %s for %s: %v", types.MANIFEST_FILE, name, err)
	}
	err = mergeDeps(&manifest)
	if err != nil {
		return nil, fmt.Errorf("Error loading %s for %s: %v", types.MANIFEST_FILE, name, err)
	}

	return &Package{
		Path:     path,
		Name:     name,
		Manifest: manifest,
	}, nil
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
		log.Errorf(context.TODO(), "Unable to create build dir in workspace: %v", err)
	}

	return buildRoot
}

func (p Package) SetupBuildDependencies(ctx context.Context, target *types.BuildTarget) error {
	for _, dep := range target.Dependencies.Build {
		if err := buildpacks.Install(ctx, p.BuildRoot(), p.Path, dep); err != nil {
			return err
		}
	}
	return nil
}

func (p Package) SetupRuntimeDependencies(ctx context.Context) error {
	for _, dep := range p.Manifest.Dependencies.Runtime {
		if err := buildpacks.Install(ctx, p.BuildRoot(), p.Path, dep); err != nil {
			return err
		}
	}
	for _, dep := range p.Manifest.Dependencies.Build {
		if err := buildpacks.Install(ctx, p.BuildRoot(), p.Path, dep); err != nil {
			return err
		}
	}
	return nil
}
