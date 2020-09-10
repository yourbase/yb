package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/yourbase/narwhal"
	. "github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
	. "github.com/yourbase/yb/types"
)

// Any flags we want to pass to the build process
type BuildFlags struct {
	HostOnly         bool
	CleanBuild       bool
	DependenciesOnly bool
	ExecPrefix       string
}

type Package struct {
	Name      string
	path      string
	Manifest  BuildManifest
	Workspace *Workspace
}

var ErrNoManifestFile = errors.New("manifest file not found")

func (p Package) Path() string {
	return p.path
}

func (p Package) Build(ctx context.Context, flags BuildFlags, targetName string) ([]CommandTimer, error) {
	times := make([]CommandTimer, 0)

	manifest := p.Manifest

	if targetName == "" {
		targetName = "default"
	}

	tgts, err := manifest.ResolveBuildTargets(targetName)
	if err != nil {
		return times, err
	}

	for _, tgt := range tgts {

		contextId := fmt.Sprintf("%s-build-%s", p.Name, tgt.Name)
		runtimeCtx := runtime.NewRuntime(ctx, contextId, p.BuildRoot())

		buildTimes, err := tgt.Build(ctx, runtimeCtx, os.Stdout, flags, p.Path(), p.Manifest.Dependencies.Build)
		if err != nil {
			return buildTimes, err
		}

		times = append(times, buildTimes...)
	}

	return times, nil
}

func LoadPackageAtPath(path string) (Package, error) {
	_, pkgName := filepath.Split(path)
	return LoadPackage(pkgName, path)
}

func (p Package) BuildRoot() string {
	return p.Workspace.BuildRoot()
}

func LoadPackage(name string, path string) (Package, error) {
	manifest := BuildManifest{}
	buildYaml := filepath.Join(path, ManifestFile)
	if _, err := os.Stat(buildYaml); os.IsNotExist(err) {
		return Package{}, ErrNoManifestFile
	}

	buildyaml, _ := ioutil.ReadFile(buildYaml)
	err := yaml.Unmarshal([]byte(buildyaml), &manifest)
	if err != nil {
		return Package{}, fmt.Errorf("loading %s for %s: %v", ManifestFile, name, err)
	}

	p := Package{
		path:     path,
		Name:     name,
		Manifest: manifest,
	}

	return p, nil
}

func (p Package) environmentVariables(ctx context.Context, data runtime.RuntimeEnvironmentData, env string) ([]string, error) {
	return p.Manifest.Exec.environmentVariables(ctx, env, data)
}

func (p Package) Execute(ctx context.Context, runtimeCtx *runtime.Runtime) error {
	return p.ExecuteToWriter(ctx, runtimeCtx, os.Stdout)
}

func (p Package) ExecuteToWriter(ctx context.Context, runtimeCtx *runtime.Runtime, output io.Writer) error {

	target, err := p.createExecutionTarget(ctx, runtimeCtx)
	if err != nil {
		return err
	}

	log.Infof("Executing package '%s'...\n", p.Name)

	env, err := p.environmentVariables(ctx, runtimeCtx.EnvironmentData(), "default")
	if err != nil {
		return err
	}

	for _, cmdString := range p.Manifest.Exec.Commands {
		proc := runtime.Process{
			Command:     cmdString,
			Directory:   "/workspace",
			Interactive: false,
			Output:      output,
			Environment: env,
		}

		if err := target.Run(ctx, proc); err != nil {
			return fmt.Errorf("unable to run command '%s': %v", cmdString, err)
		}
	}

	return nil
}

func (p Package) addMount(cd *narwhal.ContainerDefinition, localPath, remotePath, thing string) {
	if thing == "" {
		thing = "package"
	}
	log.Infof("Will mount %s %s at %s in container", thing, localPath, remotePath)
	mount := fmt.Sprintf("%s:%s", localPath, remotePath)
	cd.Mounts = append(cd.Mounts, mount)
}

func (p Package) checkMounts(cd *narwhal.ContainerDefinition, srcDir string) error {
	for _, mount := range cd.Mounts {
		parts := strings.Split(mount, ":")
		if len(parts) == 2 {
			src := filepath.Join(srcDir, parts[0])
			err := MkdirAsNeeded(src)
			return err
		}
	}
	return nil
}

func (p Package) createExecutionTarget(ctx context.Context, runtimeCtx *runtime.Runtime) (*runtime.ContainerTarget, error) {
	localContainerWorkDir := filepath.Join(p.BuildRoot(), "containers")
	MkdirAsNeeded(localContainerWorkDir)

	log.Infof("Will use %s as the dependency work dir", localContainerWorkDir)

	manifest := p.Manifest
	containers := manifest.Exec.Dependencies.ContainerList()
	for _, cd := range containers {
		cd.LocalWorkDir = localContainerWorkDir
		if err := p.checkMounts(&cd, localContainerWorkDir); err != nil {
			return nil, fmt.Errorf("set host container mount dir: %v", err)
		}

		_, err := runtimeCtx.AddContainer(ctx, cd)
		if err != nil {
			return nil, fmt.Errorf("starting container dependency: %v", err)
		}
	}

	// TODO: Support UDP
	portMappings := make([]string, 0)

	for _, entry := range p.Manifest.Exec.Ports {
		localPort := ""
		remotePort := ""
		parts := strings.Split(entry, ":")

		if len(parts) == 2 {
			localPort = parts[0]
			remotePort = parts[1]
		} else {
			if runtime.HostOS() == runtime.Linux {
				log.Infof("No host port specified for port %s - will use %s externaly", entry, entry)
				localPort = entry
			} else {
				log.Infof("Docker is not running natively, will try to pick a random port for %s", entry)
				p, err := runtime.GetFreePort()
				if err != nil {
					log.Warnf("Could not find local port for container port %s: %v", entry, err)
				} else {
					localPort = fmt.Sprintf("%d", p)
				}
			}

			remotePort = entry
		}

		if localPort != "" && remotePort != "" {
			mapString := fmt.Sprintf("%s:%s", localPort, remotePort)
			portMappings = append(portMappings, mapString)
		}

		log.Infof("Mapping container port %s to %s on the local machine", remotePort, localPort)
	}

	env, err := manifest.Exec.environmentVariables(ctx, "default", runtimeCtx.EnvironmentData())
	if err != nil {
		return nil, err
	}

	execContainer := manifest.Exec.Container
	execContainer.Environment = env
	execContainer.Command = "/usr/bin/tail -f /dev/null"
	execContainer.Label = p.Name
	execContainer.Ports = portMappings

	// Add package to mounts @ /workspace
	sourceMapDir := "/workspace"
	if execContainer.WorkDir != "" {
		sourceMapDir = execContainer.WorkDir
	}
	if err := p.checkMounts(&execContainer, p.Path()); err != nil {
		return nil, fmt.Errorf("set host container mount dir: %v", err)
	}
	p.addMount(&execContainer, p.Path(), sourceMapDir, "package")

	execTarget, err := runtimeCtx.AddContainer(ctx, execContainer)
	if err != nil {
		return nil, fmt.Errorf("starting exec container: %v", err)
	}

	LoadBuildPacks(ctx, execTarget, p.Manifest.Dependencies.Build)
	LoadBuildPacks(ctx, execTarget, p.Manifest.Dependencies.Runtime)

	return execTarget, nil
}

func (p Package) RuntimeContainers() []narwhal.ContainerDefinition {
	m := p.Manifest

	result := make([]narwhal.ContainerDefinition, 0)
	for _, c := range m.Exec.Dependencies.ContainerList() {
		result = append(result, c)
	}
	return result
}
