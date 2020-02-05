package runtime

import (
	"fmt"

	"github.com/yourbase/narwhal"
)

type Os int

const (
	Linux Os = iota
	Darwin
	Windows
)

type Architecture int

const (
	Amd64 Architecture = iota
	i386
)



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
