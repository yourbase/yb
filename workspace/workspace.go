package workspace

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/yourbase/yb/packages"
	"github.com/yourbase/yb/plumbing"
	"gopkg.in/yaml.v2"
)

type Workspace struct {
	Target string `yaml:"target"`
	Path   string
}

// Change this
func (w Workspace) Root() string {
	return w.Path
}

func (w Workspace) TargetPackage() (packages.Package, error) {
	if w.Target != "" {
		return w.PackageByName(w.Target)
	} else {
		return packages.Package{}, fmt.Errorf("No default package specified for the workspace")
	}
}

func (w Workspace) PackageByName(name string) (packages.Package, error) {
	pkgList, err := w.PackageList()

	if err != nil {
		return packages.Package{}, err
	}

	for _, pkg := range pkgList {
		if pkg.Name == name {
			return pkg, nil
		}
	}

	return packages.Package{}, fmt.Errorf("No package with name %s found in the workspace", name)
}

func (w Workspace) PackageList() ([]packages.Package, error) {
	var packagesList []packages.Package

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
				pkg, err := packages.LoadPackage(pkgName, pkgPath)
				if err != nil {
					return packagesList, err
				}
				packagesList = append(packagesList, pkg)
			}
		}
	}

	return packagesList, nil

}

func (w Workspace) BuildRoot() string {
	return filepath.Join(w.Path, "build")
}

func (w Workspace) SetupEnv() error {
	fmt.Println("Clearing environment variables!")
	criticalVariables := []string{"USER", "USERNAME", "UID", "GID", "TTY", "PWD"}
	oldEnv := make(map[string]string)
	for _, key := range criticalVariables {
		oldEnv[key] = os.Getenv(key)
	}

	os.Clearenv()
	tmpDir := filepath.Join(w.BuildRoot(), "tmp")
	plumbing.MkdirAsNeeded(tmpDir)
	os.Setenv("HOME", w.BuildRoot())
	os.Setenv("TMPDIR", tmpDir)

	for _, key := range criticalVariables {
		fmt.Printf("%s=%s\n", key, oldEnv[key])
		os.Setenv(key, oldEnv[key])
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

	workspacePath, err := plumbing.FindWorkspaceRoot()

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

	fmt.Printf("Workspace path: %s\n", workspacePath)
	workspace.Path = workspacePath
	return workspace, nil
}
