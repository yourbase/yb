package runtime

import (
	"fmt"

	"github.com/yourbase/narwhal"
	"github.com/yourbase/yb/plumbing/log"
)

type ContainerRuntime struct {
	Runtime
	ServiceContext *narwhal.ServiceContext
	Targets        map[string]*ContainerTarget
}

type ContainerTarget struct {
	Target
	Container *narwhal.Container
}

type ContainerRuntimeOpts struct {
	Identifier           string
	ContainerDefinitions []narwhal.ContainerDefinition
	LocalWorkDir         string
}

func (t *ContainerTarget) Run(p Process) error {
	if p.Interactive {
		return t.Container.ExecInteractively(p.Command, p.Directory)
	} else {
		return t.Container.ExecToStdout(p.Command, p.Directory)
	}
}

func (r *ContainerRuntime) AddTarget(targetId string, t *ContainerTarget) error {
	if _, exists := r.Targets[targetId]; exists {
		return fmt.Errorf("Unable to add target with id %s - already exists!", targetId)
	}

	r.Targets[targetId] = t

	return nil
}

func (r *ContainerRuntime) Run(p Process, targetId string) error {
	if target, exists := r.Targets[targetId]; exists {
		return target.Run(p)
	} else {
		return fmt.Errorf("Unable to find target %s in runtime!", targetId)
	}
}

func (r *ContainerRuntime) AddContainer(cd narwhal.ContainerDefinition) error {
	if container, err := r.ServiceContext.StartContainer(cd); err != nil {
		return fmt.Errorf("Couldn't start container %s in service context: %v", cd.Label, err)
	} else {
		tgt := &ContainerTarget{
			Container: container,
		}
		r.AddTarget(cd.Label, tgt)
	}
	return nil
}

func NewContainerRuntime(opts ContainerRuntimeOpts) (*ContainerRuntime, error) {

	sc, err := narwhal.NewServiceContextWithId(opts.Identifier, opts.LocalWorkDir)
	if err != nil {
		return nil, err
	}

	r := &ContainerRuntime{
		ServiceContext: sc,
		Targets:        make(map[string]*ContainerTarget),
	}

	log.Infof("Starting %d containers:", len(opts.ContainerDefinitions))

	for _, cd := range opts.ContainerDefinitions {
		log.Infof("  * %s (%s)", cd.Label, cd.ImageNameWithTag())
		if err := r.AddContainer(cd); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func (r *ContainerRuntime) Shutdown() error {
	return r.ServiceContext.TearDown()
}
