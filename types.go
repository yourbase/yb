package main

type BuildInstructions struct {
	Dependencies DependencySet `yaml:"dependencies"`
	Build        BuildPhase    `yaml:"build"`
	Exec         ExecPhase     `yaml:"exec"`
}

type DependencySet struct {
	Build   []string `yaml:"build"`
	Runtime []string `yaml:"runtime"`
}

type ExecPhase struct {
	Container    ContainerDefinition `yaml:"container"`
	Commands     []string            `yaml:"commands"`
	Ports        []string            `yaml:"ports"`
	Environment  []string            `yaml:"environment"`
	Dependencies ExecDependencies    `yaml:"dependencies"`
}

type ExecDependencies struct {
	Containers map[string]ContainerDefinition `yaml:"containers"`
}

type ContainerDefinition struct {
	Image       string   `yaml:"image"`
	Mounts      []string `yaml:"mounts"`
	Ports       []string `yaml:"ports"`
	Environment []string `yaml:"environment"`
}

type BuildPhase struct {
	Container   ContainerDefinition `yaml:"container"`
	Tools       []string            `yaml:"tools"`
	Commands    []string            `yaml:"commands"`
	Sandbox     bool                `yaml:"sandbox"`
	Artifacts   []string            `yaml:"artifacts"`
	Environment []string            `yaml:"env"`
}

// API Responses -- use Swagger instead, this is silly
type Project struct {
	Id          int    `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Repository  string `json:"repository"`
}
