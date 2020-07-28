package workspace

import (
	"errors"
	"fmt"
	"github.com/yourbase/narwhal"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	. "github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
	. "github.com/yourbase/yb/types"
)

// Any flags we want to pass to the build process
type BuildFlags struct {
	HostOnly   bool
	CleanBuild bool
	ExecPrefix string
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

func (p Package) Build(flags BuildFlags) ([]CommandTimer, error) {
	times := make([]CommandTimer, 0)

	manifest := p.Manifest

	tgt, err := manifest.BuildTarget("default")
	if err != nil {
		return times, err
	}

	contextId := fmt.Sprintf("%s-build-%s", p.Name, tgt.Name)

	runtimeCtx := runtime.NewRuntime(contextId, p.BuildRoot())

	return tgt.Build(runtimeCtx, os.Stdout, flags, p.Path(), p.Manifest.Dependencies.Build)
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
	buildYaml := filepath.Join(path, MANIFEST_FILE)
	if _, err := os.Stat(buildYaml); os.IsNotExist(err) {
		return Package{}, ErrNoManifestFile
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

func (p Package) environmentVariables(data runtime.RuntimeEnvironmentData, env string) []string {
	return p.Manifest.Exec.EnvironmentVariables(env, data)
}

func (p Package) Execute(runtimeCtx *runtime.Runtime) error {
	return p.ExecuteToWriter(runtimeCtx, os.Stdout)
}

func (p Package) ExecuteToWriter(runtimeCtx *runtime.Runtime, output io.Writer) error {

	target, err := p.createExecutionTarget(runtimeCtx)
	if err != nil {
		return err
	}

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Ctrl+C pressed in Terminal")
		if err := runtimeCtx.Shutdown(); err != nil {
			// Oops.
			os.Exit(1)
		}
		// Ok!
		os.Exit(0)
	}()

	log.Infof("Executing package '%s'...\n", p.Name)

	for _, cmdString := range p.Manifest.Exec.Commands {
		proc := runtime.Process{
			Command:     cmdString,
			Directory:   "/workspace",
			Interactive: false,
			Output:      &output,
			Environment: p.environmentVariables(runtimeCtx.EnvironmentData(), "default"),
		}

		if err := target.Run(proc); err != nil {
			return fmt.Errorf("unable to run command '%s': %v", cmdString, err)
		}
	}

	return nil
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

func (p Package) createExecutionTarget(runtimeCtx *runtime.Runtime) (*runtime.ContainerTarget, error) {
	localContainerWorkDir := filepath.Join(p.BuildRoot(), "containers")
	MkdirAsNeeded(localContainerWorkDir)

	log.Infof("Will use %s as the dependency work dir", localContainerWorkDir)

	manifest := p.Manifest
	containers := manifest.Exec.Dependencies.ContainerList()
	for _, cd := range containers {
		cd.LocalWorkDir = localContainerWorkDir
		if err := p.checkMounts(&cd, localContainerWorkDir); err != nil {
			return nil, fmt.Errorf("Unable to set host container mount dir: %v", err)
		}

		_, err := runtimeCtx.AddContainer(cd)
		if err != nil {
			return nil, fmt.Errorf("Couldn't start container dependency: %v", err)
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

	execContainer := manifest.Exec.Container
	execContainer.Environment = manifest.Exec.EnvironmentVariables("default", runtimeCtx.EnvironmentData())
	execContainer.Command = "/usr/bin/tail -f /dev/null"
	execContainer.Label = p.Name
	execContainer.Ports = portMappings

	// Add package to mounts @ /workspace
	sourceMapDir := "/workspace"
	if execContainer.WorkDir != "" {
		sourceMapDir = execContainer.WorkDir
	}
	log.Infof("Will mount package %s at %s in container", p.Path(), sourceMapDir)
	mount := fmt.Sprintf("%s:%s", p.Path(), sourceMapDir)
	execContainer.Mounts = append(execContainer.Mounts, mount)

	execTarget, err := runtimeCtx.AddContainer(execContainer)
	if err != nil {
		return nil, fmt.Errorf("Couldn't start exec container: %v", err)
	}

	LoadBuildPacks(execTarget, p.Manifest.Dependencies.Build)
	LoadBuildPacks(execTarget, p.Manifest.Dependencies.Runtime)

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
