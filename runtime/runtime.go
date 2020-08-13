package runtime

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/yourbase/yb/plumbing/log"

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
	Run(ctx context.Context, p Process) error
	SetEnv(key string, value string) error
	UploadFile(ctx context.Context, localPath string, remotePath string) error
	WriteFileContents(ctx context.Context, contents string, remotepath string) error
	WorkDir() string
	CacheDir(ctx context.Context) string
	DownloadFile(ctx context.Context, url string) (string, error)
	Unarchive(ctx context.Context, src string, dst string) error
	PrependToPath(ctx context.Context, dir string)
	PathExists(ctx context.Context, path string) bool
	GetDefaultPath() string
	ToolsDir(ctx context.Context) string
	ToolOutputSharedDir(ctx context.Context) string
	OS() Os
	OSVersion(ctx context.Context) string
	Architecture() Architecture
	MkdirAsNeeded(ctx context.Context, path string) error
}

type Process struct {
	Command     string
	Interactive bool
	Directory   string
	Environment []string
	Output      io.Writer
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

func (r *Runtime) NTargets() int {
	return len(r.Targets)
}

func (r *Runtime) AddTarget(targetId string, t Target) error {
	if _, exists := r.Targets[targetId]; exists {
		return fmt.Errorf("adding target with id %s - already exists", targetId)
	}

	r.Targets[targetId] = t

	return nil
}

func (r *Runtime) Run(ctx context.Context, p Process) error {
	return r.DefaultTarget.Run(ctx, p)
}

func (r *Runtime) RunInTarget(ctx context.Context, p Process, targetId string) error {
	if target, exists := r.Targets[targetId]; exists {
		return target.Run(ctx, p)
	} else {
		return fmt.Errorf("finding target %s in runtime", targetId)
	}
}

func (r *Runtime) FindContainers(ctx context.Context, definitions []narwhal.ContainerDefinition) []*ContainerTarget {
	result := make([]*ContainerTarget, 0)

	if r.SupportsContainers() {
		for _, cd := range definitions {
			container, err := r.ContainerServiceContext.FindContainer(ctx, &cd)
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

func (r *Runtime) AddContainer(ctx context.Context, cd narwhal.ContainerDefinition) (*ContainerTarget, error) {
	if r.SupportsContainers() {
		container, err := r.ContainerServiceContext.StartContainer(ctx, nil, &cd)
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

func NewRuntime(ctx context.Context, identifier string, localWorkDir string) *Runtime {

	sc, err := narwhal.NewServiceContextWithId(ctx, narwhal.DockerClient(), identifier, localWorkDir)
	if err != nil {
		log.Infof("Container service context failed to initialize - containers won't be supported: %v", err)
		sc = nil
	}

	return &Runtime{
		Identifier:              identifier,
		LocalWorkDir:            localWorkDir,
		Targets:                 make(map[string]Target),
		DefaultTarget:           &MetalTarget{workDir: localWorkDir},
		ContainerServiceContext: sc,
	}
}

func (r *Runtime) Shutdown(ctx context.Context) error {

	if r.ContainerServiceContext != nil {
		if err := r.ContainerServiceContext.TearDown(ctx); err != nil {
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

// Environment returns a map of enviroment variables with each container IP adress
//  it also fill up the ip map, that is used in IP
func (c ContainerData) Environment(ctx context.Context) map[string]string {
	result := make(map[string]string)
	if c.serviceCtx != nil {
		for _, containerDef := range c.serviceCtx.ContainerDefinitions {
			container, err := c.serviceCtx.FindContainer(ctx, &containerDef)
			if err != nil {
				return nil
			}
			if ipv4, err := narwhal.IPv4Address(ctx, narwhal.DockerClient(), container.Id); err == nil {
				key := fmt.Sprintf("YB_CONTAINER_%s_IP", strings.ToUpper(containerDef.Label))
				result[key] = ipv4.String()
			}
		}
	}
	return result
}

// IP returns an IP adress associated with an container "label"
func (c ContainerData) IP(label string) string {
	if c.serviceCtx != nil {
		var containerDef narwhal.ContainerDefinition
		for _, c := range c.serviceCtx.ContainerDefinitions {
			log.Debugf("Container Def label: %s; searching: %s", c.Label, label)
			if label == c.Label {
				containerDef = c
				break
			}
		}
		container, err := c.serviceCtx.FindContainer(context.TODO(), &containerDef)
		if err != nil {
			return ""
		}
		if ipv4, err := narwhal.IPv4Address(context.TODO(), c.serviceCtx.DockerClient, container.Id); err == nil {
			return ipv4.String()
		}
	}
	return ""
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
