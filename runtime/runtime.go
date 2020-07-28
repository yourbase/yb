package runtime

import (
	"fmt"
	"github.com/yourbase/yb/plumbing/log"
	"io"
	"strings"

	goruntime "runtime"

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
	Message  string
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
	GetDefaultPath() string
	ToolsDir() string
	OS() Os
	OSVersion() string
	Architecture() Architecture
}

type Process struct {
	Command     string
	Interactive bool
	Directory   string
	Environment []string
	Output      *io.Writer
}

type Runtime struct {
	Identifier              string
	LocalWorkDir            string
	Targets                 map[string]Target
	ContainerServiceContext *narwhal.ServiceContext
	DefaultTarget           Target
}

func (r *Runtime) SupportsContainers() bool {
	return r.ContainerServiceContext != nil
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

func (r *Runtime) FindContainers(definitions []narwhal.ContainerDefinition) []*ContainerTarget {
	result := make([]*ContainerTarget, 0)

	if r.SupportsContainers() {
		for _, cd := range definitions {
			container, err := r.ContainerServiceContext.FindContainer(cd)
			if err != nil {
				log.Warnf("Error trying to find container %s - %v", cd.Label, err)
			} else {
				// TODO: Env / workdir?
				tgt := &ContainerTarget{Container: container}
				result = append(result, tgt)
			}
		}
	}

	return result
}

func (r *Runtime) AddContainer(cd narwhal.ContainerDefinition) (*ContainerTarget, error) {
	if r.SupportsContainers() {
		container, err := r.ContainerServiceContext.StartContainer(cd)
		if err != nil {
			return nil, fmt.Errorf("could not start container %s: %v", cd.Label, err)
		}

		tgt := &ContainerTarget{
			Container: container,
		}

		if err = r.AddTarget(cd.Label, tgt); err != nil {
			return nil, err
		}

		return tgt, nil
	} else {
		return nil, fmt.Errorf("current runtime does not support containers")
	}
}

func NewRuntime(identifier string, localWorkDir string) *Runtime {

	sc, err := narwhal.NewServiceContextWithId(identifier, localWorkDir)
	if err != nil {
		log.Infof("Container service context failed to initialize - containers won't be supported: %v", err)
		sc = nil
	}

	return &Runtime{
		Identifier:              identifier,
		LocalWorkDir:            localWorkDir,
		Targets:                 make(map[string]Target),
		DefaultTarget:           &MetalTarget{},
		ContainerServiceContext: sc,
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
		Services: ServiceData{
			serviceCtx: r.ContainerServiceContext,
		},
	}
}

type RuntimeEnvironmentData struct {
	Containers ContainerData
	Services   ServiceData
}

type ServiceData struct {
	serviceCtx *narwhal.ServiceContext
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

func (c ServiceData) IP(label string) string {
	// Check service context
	log.Debugf("Looking up IP for service %s", label)
	if c.serviceCtx != nil {
		if buildContainer, ok := c.serviceCtx.Containers[label]; ok {
			log.Debugf("Found service %s with container %s", label, buildContainer.Id)
			if ipv4, err := buildContainer.IPv4Address(); err == nil {
				log.Debugf("Service %s has IP %s", label, ipv4)
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

func HostOS() Os {
	switch goruntime.GOOS {
	case "darwin":
		return Darwin
	case "linux":
		return Linux
	case "windows":
		return Windows
	default:
		return Unknown
	}
}
