package workspace

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/yourbase/yb/runtime"
	"gopkg.in/yaml.v2"

	. "github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
)

type Workspace struct {
	Target string `yaml:"target"`
	Path   string
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
	pkgList, err := w.PackageList()

	if err != nil {
		return Package{}, err
	}

	for _, pkg := range pkgList {
		if pkg.Name == name {
			return pkg, nil
		}
	}

	return Package{}, fmt.Errorf("No package with name %s found in the workspace", name)
}

func (w Workspace) PackageList() ([]Package, error) {
	var packages []Package

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
			_, pkgName := filepath.Split(f)
			pkgPath := f
			if !strings.HasPrefix(pkgName, ".") {
				pkg, err := LoadPackage(pkgName, pkgPath)
				if err != nil {
					if err == ErrNoManifestFile {
						log.Debugf("No manifest file found for %s", pkgName)
					} else {
						log.Errorf("Error loading package '%s': %v", pkgName, err)
						return packages, err
					}
				} else {
					packages = append(packages, pkg)
				}
			}
		}
	}

	return packages, nil

}

func (w Workspace) BuildRoot() string {
	return filepath.Join(w.Path, "build")
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

func LoadWorkspace() (Workspace, error) {

	workspacePath, err := FindWorkspaceRoot()

	if err != nil {
		return Workspace{}, fmt.Errorf("Error getting workspace path: %v", err)
	}

	var workspace = Workspace{}
	configFile := filepath.Join(workspacePath, "config.yml")
	configyaml, _ := ioutil.ReadFile(configFile)
	err = yaml.Unmarshal([]byte(configyaml), &workspace)

	if err != nil {
		return Workspace{}, fmt.Errorf("Error loading workspace config!")
	}

	log.Infof("Workspace path: %s\n", workspacePath)
	workspace.Path = workspacePath
	return workspace, nil
}
