package runtime

type Target interface {
	Run(p Process) error
}

type Process struct {
	Command     string
	Interactive bool
	Directory   string
}

type RuntimeOpts struct {
	Identifier string
}

type Runtime interface {
	AddTarget(targetId string, t Target) error
	Run(p Process, targetId string) error
	Shutdown() error
}
