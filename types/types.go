package types

import (
	"github.com/alexcesaro/log/stdlog"

	"time"
)

const (
	MANIFEST_FILE = ".yourbase.yml"
	DOCS_URL      = "https://docs.yourbase.io"
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
	WorkDir     string   `yaml:"workdir"`
	Privileged  bool
}

type BuildTarget struct {
	Name        string              `yaml:"name"`
	Container   ContainerDefinition `yaml:"container"`
	Tools       []string            `yaml:"tools"`
	Commands    []string            `yaml:"commands"`
	Artifacts   []string            `yaml:"artifacts"`
	CachePaths  []string            `yaml:"cache_paths"`
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
	OrgSlug     string `json:"org_slug"`
}

type TokenResponse struct {
	TokenOK bool `json:"token_ok"`
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

// Logger used by everyone
var LOGGER = stdlog.GetFromFlags()
