package runtime

import (
	"fmt"
	"strings"

	"github.com/yourbase/narwhal"
)

type Os int

const (
	Linux Os = iota
	Darwin
	Windows
	Unknown
)

type Architecture int

const (
	Amd64 Architecture = iota
	i386
)

type TargetRunError struct {
	ExitCode int
	Message string
}

func (e *TargetRunError) Error() string {
	return e.Message
}


type Target interface {
	Run(p Process) error
	SetEnv(key string, value string) error
	UploadFile(localPath string, remotePath string) error
	WriteFileContents(contents string, remotepath string) error
	WorkDir() string
	CacheDir() string
	DownloadFile(url string) (string, error)
	Unarchive(src string, dst string) error
	PrependToPath(dir string)
	PathExists(path string) bool
	ToolsDir() string
	OS() Os
	Architecture() Architecture
}

type Process struct {
	Command     string
	Interactive bool
	Directory   string
	Environment []string
}

type Runtime struct {
	Identifier              string
	LocalWorkDir            string
	Targets                 map[string]Target
	ContainerServiceContext *narwhal.ServiceContext
	DefaultTarget           Target
}

func (r *Runtime) AddTarget(targetId string, t Target) error {
	if _, exists := r.Targets[targetId]; exists {
		return fmt.Errorf("Unable to add target with id %s - already exists", targetId)
	}

	r.Targets[targetId] = t

	return nil
}

func (r *Runtime) Run(p Process) error {
	return r.DefaultTarget.Run(p)
}

func (r *Runtime) RunInTarget(p Process, targetId string) error {
	if target, exists := r.Targets[targetId]; exists {
		return target.Run(p)
	} else {
		return fmt.Errorf("Unable to find target %s in runtime", targetId)
	}
}

func (r *Runtime) AddContainer(cd narwhal.ContainerDefinition) (*ContainerTarget, error) {
	if r.ContainerServiceContext == nil {
		sc, err := narwhal.NewServiceContextWithId(r.Identifier, r.LocalWorkDir)
		if err != nil {
			return nil, err
		}
		r.ContainerServiceContext = sc
	}

	container, err := r.ContainerServiceContext.StartContainer(cd)
	if err != nil {
		return nil, fmt.Errorf("Couldn't start container %s in service context: %v", cd.Label, err)
	}

	tgt := &ContainerTarget{
		Container: container,
	}

	r.AddTarget(cd.Label, tgt)
	return tgt, nil
}

func NewRuntime(identifier string, localWorkDir string) *Runtime {
	return &Runtime{
		Identifier:    identifier,
		LocalWorkDir:  localWorkDir,
		Targets:       make(map[string]Target),
		DefaultTarget: &MetalTarget{},
	}
}

func (r *Runtime) Shutdown() error {

	if r.ContainerServiceContext != nil {
		if err := r.ContainerServiceContext.TearDown(); err != nil {
			return err
		}
	}

	return nil
}

func (r *Runtime) EnvironmentData() RuntimeEnvironmentData {
	return RuntimeEnvironmentData{
		Containers: ContainerData{
			serviceCtx: r.ContainerServiceContext,
		},
	}
}

type RuntimeEnvironmentData struct {
	Containers ContainerData
}

type ContainerData struct {
	serviceCtx *narwhal.ServiceContext
}

func (c ContainerData) IP(label string) string {
	// Check service context
	if c.serviceCtx != nil {
		if buildContainer, ok := c.serviceCtx.Containers[label]; ok {
			if ipv4, err := buildContainer.IPv4Address(); err == nil {
				return ipv4
			}
		}
	}

	return ""
}

func (c ContainerData) Environment() map[string]string {
	result := make(map[string]string)
	if c.serviceCtx != nil {
		for label, container := range c.serviceCtx.Containers {
			if ipv4, err := container.IPv4Address(); err == nil {
				key := fmt.Sprintf("YB_CONTAINER_%s_IP", strings.ToUpper(label))
				result[key] = ipv4
			}
		}
	}
	return result
}

