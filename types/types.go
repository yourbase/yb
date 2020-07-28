package types

import (
	"time"

	"github.com/johnewart/narwhal"
)

const (
	MANIFEST_FILE        = ".yourbase.yml"
	DOCS_URL             = "https://docs.yourbase.io"
	DEFAULT_YB_CONTAINER = "yourbase/yb_ubuntu:18.04"
)

type BuildManifest struct {
	Dependencies DependencySet `yaml:"dependencies"`
	Sandbox      bool          `yaml:"sandbox"`
	BuildTargets []BuildTarget `yaml:"build_targets"`
	Build        BuildTarget   `yaml:"build"`
	Exec         ExecPhase     `yaml:"exec"`
	Package      PackagePhase  `yaml:"package"`
	CI           CIInfo        `yaml:"ci"`
}

type CIInfo struct {
	CIBuilds []CIBuild `yaml:"builds"`
}

type CIBuild struct {
	Name        string `yaml:"name"`
	BuildTarget string `yaml:"build_target"`
	When        string `yaml:"when"`
}

type PackagePhase struct {
	Artifacts []string `yaml:"artifacts"`
}

type DependencySet struct {
	Build   []string `yaml:"build"`
	Runtime []string `yaml:"runtime"`
}

type ExecPhase struct {
	Name         string                      `yaml:"name"`
	Dependencies ExecDependencies            `yaml:"dependencies"`
	Container    narwhal.ContainerDefinition `yaml:"container"`
	Commands     []string                    `yaml:"commands"`
	Ports        []string                    `yaml:"ports"`
	Environment  map[string][]string         `yaml:"environment"`
	LogFiles     []string                    `yaml:"logfiles"`
	Sandbox      bool                        `yaml:"sandbox"`
	HostOnly     bool                        `yaml:"host_only"`
	BuildFirst   []string                    `yaml:"build_first"`
}

type BuildDependencies struct {
	Build      []string                               `yaml:"build"`
	Containers map[string]narwhal.ContainerDefinition `yaml:"containers"`
}

func (b BuildDependencies) ContainerList() []narwhal.ContainerDefinition {
	containers := make([]narwhal.ContainerDefinition, 0)
	for label, c := range b.Containers {
		c.Label = label
		containers = append(containers, c)
	}
	return containers
}

type ExecDependencies struct {
	Containers map[string]narwhal.ContainerDefinition `yaml:"containers"`
}

func (b ExecDependencies) ContainerList() []narwhal.ContainerDefinition {
	containers := make([]narwhal.ContainerDefinition, 0)
	for label, c := range b.Containers {
		c.Label = label
		containers = append(containers, c)
	}
	return containers
}

type BuildTarget struct {
	Name         string                      `yaml:"name"`
	Container    narwhal.ContainerDefinition `yaml:"container"`
	Tools        []string                    `yaml:"tools"`
	Commands     []string                    `yaml:"commands"`
	Artifacts    []string                    `yaml:"artifacts"`
	CachePaths   []string                    `yaml:"cache_paths"`
	Sandbox      bool                        `yaml:"sandbox"`
	HostOnly     bool                        `yaml:"host_only"`
	Root         string                      `yaml:"root"`
	Environment  []string                    `yaml:"environment"`
	Tags         map[string]string           `yaml:"tags"`
	BuildAfter   []string                    `yaml:"build_after"`
	Dependencies BuildDependencies           `yaml:"dependencies"`
}

// API Responses -- TODO use Swagger instead, this is silly
type Project struct {
	Id          int    `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Repository  string `json:"repository"`
	OrgSlug     string `json:"organization_slug"`
}

type TokenResponse struct {
	TokenOK bool `json:"token_ok"`
}

type WorktreeSave struct {
	Hash    string
	Path    string
	Files   []string
	Enabled bool
}

type CommandTimer struct {
	Command   string
	StartTime time.Time
	EndTime   time.Time
}

type TargetTimer struct {
	Name   string
	Timers []CommandTimer
}

type BuildTool interface {
	Install() error
	Setup() error
	Version() string
}
