package runtime

import (
	"github.com/yourbase/yb/plumbing"
)

type MetalRuntime struct {
	Runtime
}

type MetalTarget struct {
	Target
}

func (t *MetalTarget) Run(p Process) error {
	return plumbing.ExecToStdout(p.Command, "/")
}

func (r *MetalRuntime) AddTarget(targetId string, t *MetalTarget) error {
	// Do nothing
	return nil
}

func (r *MetalRuntime) Run(p Process, targetId string) error {
	return plumbing.ExecToStdout(p.Command, "/")
}

func NewMetalRuntime() (*MetalRuntime, error) {
	r := &MetalRuntime{}
	return r, nil
}

func (r *MetalRuntime) Shutdown() error {
	return nil
}
