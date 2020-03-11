package workspace

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/yourbase/yb/runtime"
	"gopkg.in/yaml.v2"

	. "github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
)

type Workspace struct {
	Target    string `yaml:"target"`
	Path      string
	BuildSpec *BuildSpec
	packages  []Package
}

// Change this
func (w Workspace) Root() string {
	return w.Path
}

func (w Workspace) TargetPackage() (Package, error) {
	if w.Target != "" {
		return w.PackageByName(w.Target)
	} else {
		return Package{}, fmt.Errorf("No default package specified for the workspace")
	}
}

func (w Workspace) PackageByName(name string) (Package, error) {
	for _, pkg := range w.packages {
		if pkg.Name == name {
			return pkg, nil
		}
	}

	return Package{}, fmt.Errorf("No package with name %s found in the workspace", name)
}

func (w Workspace) PackageList() []Package {
	return w.packages
}

func (w Workspace) BuildRoot() string {
	buildDir := "build"
	buildRoot := filepath.Join(w.Path, buildDir)

	if _, err := os.Stat(buildRoot); os.IsNotExist(err) {
		if err := os.Mkdir(buildRoot, 0700); err != nil {
			log.Warnf("Unable to create build dir in workspace: %v\n", err)
		}
	}

	return buildRoot
}

func (w Workspace) SetupEnv() error {
	log.Infof("Resetting environment variables!")
	criticalVariables := []string{"USER", "USERNAME", "UID", "GID", "TTY", "PWD"}
	oldEnv := make(map[string]string)
	for _, key := range criticalVariables {
		oldEnv[key] = os.Getenv(key)
	}

	os.Clearenv()
	tmpDir := filepath.Join(w.BuildRoot(), "tmp")
	MkdirAsNeeded(tmpDir)
	runtime.SetEnv("HOME", w.BuildRoot())
	runtime.SetEnv("TMPDIR", tmpDir)

	for _, key := range criticalVariables {
		log.Infof("%s=%s\n", key, oldEnv[key])
		runtime.SetEnv(key, oldEnv[key])
	}

	return nil
}

func (w Workspace) Save() error {
	d, err := yaml.Marshal(w)
	if err != nil {
		log.Fatalf("error: %v", err)
		return err
	}
	err = ioutil.WriteFile(filepath.Join(w.Path, "config.yml"), d, 0644)
	if err != nil {
		log.Fatalf("Unable to write config: %v", err)
		return err
	}
	return nil
}

func loadWorkspaceWithSpecFile(specFile string, path string) (Workspace, error) {
	workspace := Workspace{Path: path}

	spec, err := LoadBuildSpec(specFile)
	if err != nil {
		log.Errorf("Unable to parse config: %v", err)
	} else {
		workspace.BuildSpec = &spec
		for _, s := range spec.Targets() {
			path := filepath.Join(path, s.Name)
			if mf, err := spec.GenerateManifest(s.Name); err != nil {
				log.Warnf("Unable to load package %s: %v", s.Name, err)
			} else {
				pkg := Package{
					Name:      s.Name,
					path:      path,
					Manifest:  mf,
					Workspace: &workspace,
				}
				workspace.packages = append(workspace.packages, pkg)
			}
		}
	}

	return workspace, nil
}

func loadWorkspaceFromPackage(manifestFile string, path string) (Workspace, error) {
	workspace := Workspace{}
	pkg, err := LoadPackageAtPath(path)
	pkg.Workspace = &workspace

	if err != nil {
		return workspace, fmt.Errorf("Couldn't load package for workspace: %v", err)
	}

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

	h.Write([]byte(path))
	workspaceHash := fmt.Sprintf("%x", h.Sum(nil))
	workspace.Path = filepath.Join(workspacesRoot, workspaceHash[0:12])
	workspace.packages = []Package{pkg}
	workspace.Target = pkg.Name

	return workspace, nil
}

func loadWorkspaceFromConfigYaml(configFile string, path string) (Workspace, error) {
	workspace := Workspace{}
	configyaml, _ := ioutil.ReadFile(configFile)
	err := yaml.Unmarshal([]byte(configyaml), &workspace)

	if err != nil {
		return Workspace{}, fmt.Errorf("Error loading workspace config!")
	}

	workspace.Path = path

	// Always load packages
	globStr := filepath.Join(path, "*")
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
			_, pkgName := filepath.Split(f)
			pkgPath := f
			if !strings.HasPrefix(pkgName, ".") {
				pkg, err := LoadPackage(pkgName, pkgPath)
				if err != nil {
					if err == ErrNoManifestFile {
						log.Debugf("No manifest file found for %s", pkgName)
					} else {
						log.Errorf("Error loading package '%s': %v", pkgName, err)
					}
				} else {
					pkg.Workspace = &workspace
					workspace.packages = append(workspace.packages, pkg)
				}
			}
		}
	}

	return workspace, nil
}

func LoadWorkspace() (Workspace, error) {

	workspacePath, err := FindWorkspaceRoot()

	if err != nil {
		return Workspace{}, fmt.Errorf("Error getting workspace path: %v", err)
	}

	specFile := filepath.Join(workspacePath, "yourbase.hcl")
	manifestFile := filepath.Join(workspacePath, ".yourbase.yml")
	configFile := filepath.Join(workspacePath, "config.yml")

	if PathExists(specFile) {
		log.Debugf("Loading workspace from spec file: %s", specFile)
		return loadWorkspaceWithSpecFile(specFile, workspacePath)
	}

	if PathExists(manifestFile) {
		log.Debugf("Loading workspace from manifest file: %s", manifestFile)
		return loadWorkspaceFromPackage(manifestFile, workspacePath)
	}

	if PathExists(configFile) {
		log.Debugf("Loading workspace from config file: %s", configFile)
		return loadWorkspaceFromConfigYaml(configFile, workspacePath)
	}

	return Workspace{}, fmt.Errorf("Error finding workspace at path %s", workspacePath)
}
