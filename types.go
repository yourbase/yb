package main

const MANIFEST_FILE = ".yourbase.yml"

type BuildManifest struct {
	Dependencies DependencySet `yaml:"dependencies"`
	Sandbox      bool          `yaml:"sandbox"`
	BuildTargets []BuildTarget `yaml:"build_targets"`
	Build        BuildTarget   `yaml:"build"`
	Exec         ExecPhase     `yaml:"exec"`
	Package      PackagePhase  `yaml:"package"`
}

type PackagePhase struct {
	Artifacts []string `yaml:"artifacts"`
}

type DependencySet struct {
	Build   []string `yaml:"build"`
	Runtime []string `yaml:"runtime"`
}

type ExecPhase struct {
	Name         string              `yaml:"name"`
	Dependencies ExecDependencies    `yaml:"dependencies"`
	Container    ContainerDefinition `yaml:"container"`
	Commands     []string            `yaml:"commands"`
	Ports        []string            `yaml:"ports"`
	Environment  map[string][]string `yaml:"environment"`
	LogFiles     []string            `yaml:"logfiles"`
	Sandbox      bool                `yaml:"sandbox"`
	BuildFirst   []string            `yaml:"build_first"`
}

type ExecDependencies struct {
	Containers map[string]ContainerDefinition `yaml:"containers"`
}

type ContainerDefinition struct {
	Image       string   `yaml:"image"`
	Mounts      []string `yaml:"mounts"`
	Ports       []string `yaml:"ports"`
	Environment []string `yaml:"environment"`
	Command     string   `yaml:"command"`
}

type BuildTarget struct {
	Name        string              `yaml:"name"`
	Container   ContainerDefinition `yaml:"container"`
	Tools       []string            `yaml:"tools"`
	Commands    []string            `yaml:"commands"`
	Artifacts   []string            `yaml:"artifacts"`
	Sandbox     bool                `yaml:"sandbox"`
	Root        string              `yaml:"root"`
	Environment []string            `yaml:"environment"`
	Tags        map[string]string   `yaml:"tags"`
	BuildAfter  []string            `yaml:"build_after"`
}

// API Responses -- use Swagger instead, this is silly
type Project struct {
	Id          int    `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Repository  string `json:"repository"`
}

type LoginResponse struct {
	UserId   int    `json:"user_id"`
	ApiToken string `json:"api_key"`
}
