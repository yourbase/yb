package workspace

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"

	. "github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
	. "github.com/yourbase/yb/types"
)

// Any flags we want to pass to the build process
type BuildFlags struct {
	HostOnly   bool
	CleanBuild bool
}

type Package struct {
	Name      string
	path      string
	Manifest  BuildManifest
	Workspace *Workspace
}

func (p Package) PackageDependencies() ([]Package, error) {
	return []Package{}, nil
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

	return tgt.Build(runtimeCtx, flags, p.Path(), p.Manifest.Dependencies.Build)
}

func (p Package) BuildTargetToWriter(buildTargetName string, flags BuildFlags, output io.Writer) ([]CommandTimer, error) {
	manifest := p.Manifest

	// Named target, look for that and resolve it
	_, err := manifest.ResolveBuildTargets(buildTargetName)

	if err != nil {
		log.Errorf("Could not compute build target '%s': %v", buildTargetName, err)
		log.Errorf("Valid build targets: %s", strings.Join(manifest.BuildTargetList(), ", "))
		return []CommandTimer{}, err
	}

	primaryTarget, err := manifest.BuildTarget(buildTargetName)
	if err != nil {
		log.Errorf("Couldn't get primary build target '%s' specs: %v", buildTargetName, err)
		return []CommandTimer{}, err
	}

	contextId := fmt.Sprintf("%s-build-%s", p.Name, primaryTarget.Name)

	runtimeCtx := runtime.NewRuntime(contextId, p.BuildRoot())

	return primaryTarget.Build(runtimeCtx, flags, p.Path(), p.Manifest.Dependencies.Build)
}

func (p Package) BuildTarget(name string, flags BuildFlags) ([]CommandTimer, error) {
	return p.BuildTargetToWriter(name, flags, os.Stdout)
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

func (p Package) DoExecute(environment string) error {
	execRuntime, err := p.ExecutionRuntime(environment)
	if err != nil {
		return err
	}

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Ctrl+C pressed in Terminal")
		execRuntime.Shutdown()
		os.Exit(0)
	}()

	log.Infof("Executing package '%s'...\n", p.Name)

	return p.Execute(execRuntime)
}

func (p Package) Execute(runtimeCtx *runtime.Runtime) error {
	return p.ExecuteToWriter(runtimeCtx, os.Stdout)
}

func (p Package) EnvironmentVariables(data runtime.RuntimeEnvironmentData, env string) []string {
	return p.Manifest.Exec.EnvironmentVariables(env, data)
}

func (p Package) ExecuteToWriter(runtimeCtx *runtime.Runtime, output io.Writer) error {

	svcDef := p.Workspace.BuildSpec.Service(p.Name)
	if svcDef != nil {
		if svcDef.DependsOn != nil {
			log.Infof("Starting dependencies for %s...", p.Name)
			for _, dep := range (*svcDef.DependsOn) {
				parts := strings.Split(dep, ".")
				if len(parts) == 2 {
					dType := parts[0]
					dName := parts[1]

					if dType == "service" {
						dPkg, err := p.Workspace.PackageByName(dName)
						if err != nil {
							return fmt.Errorf("could not start dependency '%s': %v", dName, err)
						}
						// XXX -- OMG SO BAD
						go func() {
							log.Infof("* '%s'...", dName)
							_, err := dPkg.ExecutionTarget(runtimeCtx, dName)
							if err != nil {
								log.Errorf("Unable to start dependency: %v", err)
							}
							dPkg.Execute(runtimeCtx)
						}()
					} else {
						log.Warnf("Unknown dependency type: %s", dType)
					}
				} else {
					log.Warnf("Unknown dependency format: %s", dep)
				}
			}
		}
	}

	// XXX -- OMG WTF SLEEP
	log.Infof("Waiting for dependencies to start so we can get their IPs...")
	time.Sleep(20 * time.Second)

	for _, cmdString := range p.Manifest.Exec.Commands {
		proc := runtime.Process{
			Command:     cmdString,
			Directory:   "/workspace",
			Interactive: false,
			Output:      &output,
			Environment: p.EnvironmentVariables(runtimeCtx.EnvironmentData(), "default"),
		}

		if err := runtimeCtx.RunInTarget(proc, p.Name); err != nil {
			return fmt.Errorf("Unable to run command '%s': %v", cmdString, err)
		}
	}

	return nil
}

func (p Package) ExecutionTarget(runtimeCtx *runtime.Runtime, targetName string) (*runtime.ContainerTarget, error) {
	localContainerWorkDir := filepath.Join(p.BuildRoot(), "containers")
	MkdirAsNeeded(localContainerWorkDir)

	log.Infof("Will use %s as the dependency work dir", localContainerWorkDir)

	manifest := p.Manifest
	containers := manifest.Exec.Dependencies.ContainerList()
	for _, cd := range containers {
		cd.LocalWorkDir = localContainerWorkDir
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
	//execContainer.Environment = manifest.Exec.EnvironmentVariables("default", runtimeCtx.EnvironmentData())
	execContainer.Command = "/usr/bin/tail -f /dev/null"
	execContainer.Label = targetName
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

func (p Package) ExecutionRuntime(environment string) (*runtime.Runtime, error) {
	contextId := fmt.Sprintf("%s-exec", p.Name)

	runtimeCtx := runtime.NewRuntime(contextId, p.BuildRoot())

	execTarget, err:= p.ExecutionTarget(runtimeCtx, p.Name)
	if err != nil {
		return nil, err
	}

	runtimeCtx.DefaultTarget = execTarget

	return runtimeCtx, nil
}
