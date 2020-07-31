package workspace

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"

	. "github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
	. "github.com/yourbase/yb/types"
)

type Workspace struct {
	Target   string `yaml:"target"`
	Path     string
	packages []Package
}

func (w Workspace) RunInTarget(ctx context.Context, cmdString string, workDir string, targetName string) error {

	log.Infof("Running %s in %s from %s", cmdString, targetName, workDir)

	pkg, err := w.TargetPackage()
	if err != nil {
		return err
	}

	p := runtime.Process{Command: cmdString, Interactive: true, Directory: workDir, Environment: pkg.Manifest.Exec.Environment["default"]}

	runCtx, err := w.execRuntimeContext(ctx)
	if err != nil {
		return err
	}

	_, err = pkg.createExecutionTarget(ctx, runCtx)
	if err != nil {
		return err

	}
	return runCtx.RunInTarget(ctx, p, targetName)
}

func (w Workspace) RunningContainers(ctx context.Context) ([]*runtime.ContainerTarget, error) {
	runtimeCtx, err := w.execRuntimeContext(ctx)

	if err != nil {
		return []*runtime.ContainerTarget{}, fmt.Errorf("unable to determine running containers: %v", err)
	}

	result := make([]*runtime.ContainerTarget, 0)
	for _, p := range w.PackageList() {
		for _, c := range runtimeCtx.FindContainers(ctx, p.RuntimeContainers()) {
			result = append(result, c)
		}
	}

	return result, nil
}

func (w Workspace) ExecutePackage(ctx context.Context, p Package) error {
	if runtimeCtx, err := w.execRuntimeContext(ctx); err != nil {
		return err
	} else {
		return p.Execute(ctx, runtimeCtx)
	}
}

func (w Workspace) execRuntimeContext(ctx context.Context) (*runtime.Runtime, error) {
	log.Debugf("Creating runtime context for workspace %s", w.Name())
	contextId := fmt.Sprintf("%s-exec", w.Name())

	runtimeCtx := runtime.NewRuntime(ctx, contextId, w.BuildRoot())

	return runtimeCtx, nil
}

func (w Workspace) Name() string {
	_, name := filepath.Split(w.Path)
	return name
}

// Change this
func (w Workspace) Root() string {
	return w.Path
}

func (w Workspace) TargetPackage() (Package, error) {
	if w.Target != "" {
		return w.PackageByName(w.Target)
	} else {
		return Package{}, fmt.Errorf("no default package specified for the workspace")
	}
}

func (w Workspace) PackageByName(name string) (Package, error) {
	for _, pkg := range w.packages {
		if pkg.Name == name {
			return pkg, nil
		}
	}

	return Package{}, fmt.Errorf("no package with name %s found in the workspace", name)
}

func (w Workspace) PackageList() []Package {
	return w.packages
}

func (w Workspace) BuildRoot() string {
	buildDir := "build"
	buildRoot := filepath.Join(w.Path, buildDir)

	if _, err := os.Stat(buildRoot); os.IsNotExist(err) {
		if err := os.MkdirAll(buildRoot, 0700); err != nil {
			log.Warnf("Unable to create build dir in workspace: %v\n", err)
		}
	}

	return buildRoot
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

func loadWorkspaceFromPackage(manifestFile string, path string) (Workspace, error) {
	workspace := Workspace{}
	pkg, err := LoadPackageAtPath(path)
	pkg.Workspace = &workspace

	if err != nil {
		return workspace, fmt.Errorf("loading package for workspace: %v", err)
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
		return Workspace{}, fmt.Errorf("loading workspace config!")
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
	return loadWorkspace()
}

func loadWorkspace() (Workspace, error) {

	workspacePath, err := findWorkspaceRoot()

	if err != nil {
		return Workspace{}, fmt.Errorf("getting workspace path: %v", err)
	}

	manifestFile := filepath.Join(workspacePath, ".yourbase.yml")
	configFile := filepath.Join(workspacePath, "config.yml")

	var w Workspace

	if PathExists(manifestFile) {
		log.Debugf("Loading workspace from manifest file: %s", manifestFile)
		w, err = loadWorkspaceFromPackage(manifestFile, workspacePath)
		if err != nil {
			return Workspace{}, fmt.Errorf("loading manifest file %s: %v", manifestFile, err)
		}
		if err = validWorkspaceConfig(w); err != nil {
			return Workspace{}, err
		}
		return w, nil
	}

	if PathExists(configFile) {
		log.Debugf("Loading workspace from config file: %s", configFile)
		w, err = loadWorkspaceFromConfigYaml(configFile, workspacePath)
		if err != nil {
			return Workspace{}, fmt.Errorf("unable to workspace config file: %s", configFile)
		}
		if err = validWorkspaceConfig(w); err != nil {
			return Workspace{}, err
		}
		return w, nil
	}

	return Workspace{}, fmt.Errorf("resolving workspace at path %s", workspacePath)
}

// validWorkspaceConfig will check if every expected part of whole Configuration Graph is initialized
// as expected
func validWorkspaceConfig(workspace Workspace) error {
	var valid bool
	if valid = len(workspace.packages) > 0; !valid {
		return fmt.Errorf("no packages in this workspace")
	}

	if valid = filepath.IsAbs(workspace.Path); !valid {
		return fmt.Errorf("workspace path isn't absolute: %s", workspace.Path)
	}

	if valid = workspace.Target != ""; !valid {
		return fmt.Errorf("workspace target is empty")
	}

	for _, pkg := range workspace.packages {
		if valid = pkg.Name != ""; !valid {
			return fmt.Errorf("package name is empty")
		}

		if valid = filepath.IsAbs(pkg.Path()); !valid {
			return fmt.Errorf("package %s path isn't absolute: %s", pkg.Name, pkg.Path())
		}

		if valid = len(pkg.Manifest.Exec.Commands) > 0; !valid {
			if pkg.Manifest.Exec.Environment == nil {
				pkg.Manifest.Exec.Environment = make(map[string][]string)
			}
			// No Exec Phase defined, not required, but `yb run should still work`
			pkg.Manifest.Exec.Environment["default"] = pkg.Manifest.Build.Environment
			if len(pkg.Manifest.Exec.Environment["default"]) == 0 {
				if valid = len(pkg.Manifest.BuildTargets) > 0; !valid {
					return fmt.Errorf("no exec nor build target defined, won't be able to `yb run` or `yb exec`")
				}
				pkg.Manifest.Exec.Environment["default"] = pkg.Manifest.BuildTargets[0].Environment
			}
		}

	}

	return nil
}

// findWorkspaceRoot looks in the directory above the manifest file, if there's a config.yml, use that
// otherwise we use the directory of the manifest file as the workspace root
func findWorkspaceRoot() (string, error) {
	wd, err := os.Getwd()

	if err != nil {
		panic(err)
	}

	if _, err := os.Stat(filepath.Join(wd, "config.yml")); err == nil {
		// If we're currently in the directory with the config.yml
		return wd, nil
	}

	// Look upwards to find a manifest file
	packageDir, err := findNearestManifestFile()
	if err != nil {
		return "", err
	}

	// If we find a manifest file, check the parent directory for a config.yml
	parent := filepath.Dir(packageDir)
	if _, err := os.Stat(filepath.Join(parent, "config.yml")); err == nil {
		return parent, nil
	} else {
		return packageDir, nil
	}

	// No config in the parent of the package? No workspace!
	return "", fmt.Errorf("searching for a workspace root")
}

func findFileUpTree(filename string) (string, error) {
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
			return "", fmt.Errorf("searching %s, ended up at the root", filename)
		}
	}
}

func findNearestManifestFile() (string, error) {
	return findFileUpTree(ManifestFile)
}
