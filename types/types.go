package types

import (
	"github.com/alexcesaro/log/stdlog"

	"fmt"
	"strings"
	"time"
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
	Name         string              `yaml:"name"`
	Dependencies ExecDependencies    `yaml:"dependencies"`
	Container    ContainerDefinition `yaml:"container"`
	Commands     []string            `yaml:"commands"`
	Ports        []string            `yaml:"ports"`
	Environment  map[string][]string `yaml:"environment"`
	LogFiles     []string            `yaml:"logfiles"`
	Sandbox      bool                `yaml:"sandbox"`
	HostOnly     bool                `yaml:"host_only"`
	BuildFirst   []string            `yaml:"build_first"`
}

type BuildDependencies struct {
	Containers map[string]ContainerDefinition `yaml:"containers"`
}

type ExecDependencies struct {
	Containers map[string]ContainerDefinition `yaml:"containers"`
}

type ContainerDefinition struct {
	Image         string   `yaml:"image"`
	Mounts        []string `yaml:"mounts"`
	Ports         []string `yaml:"ports"`
	Environment   []string `yaml:"environment"`
	Command       string   `yaml:"command"`
	WorkDir       string   `yaml:"workdir"`
	Privileged    bool
	PortWaitCheck PortWaitCheck `yaml:"port_check"`
	Label         string        `yaml:"label"`
}

func (c ContainerDefinition) ImageNameWithTag() string {
	return fmt.Sprintf("%s:%s", c.ImageName(), c.ImageTag())
}

func (c ContainerDefinition) ImageName() string {
	parts := strings.Split(c.Image, ":")
	return parts[0]
}

func (c ContainerDefinition) ImageTag() string {
	parts := strings.Split(c.Image, ":")
	if len(parts) != 2 {
		return "latest"
	} else {
		return parts[1]
	}
}

type PortWaitCheck struct {
	Port    int `yaml:"port"`
	Timeout int `yaml:"timeout"`
}

type BuildTarget struct {
	Name         string              `yaml:"name"`
	Container    ContainerDefinition `yaml:"container"`
	Tools        []string            `yaml:"tools"`
	Commands     []string            `yaml:"commands"`
	Artifacts    []string            `yaml:"artifacts"`
	CachePaths   []string            `yaml:"cache_paths"`
	Sandbox      bool                `yaml:"sandbox"`
	HostOnly     bool                `yaml:"host_only"`
	Root         string              `yaml:"root"`
	Environment  []string            `yaml:"environment"`
	Tags         map[string]string   `yaml:"tags"`
	BuildAfter   []string            `yaml:"build_after"`
	Dependencies BuildDependencies   `yaml:"dependencies"`
}

// API Responses -- TODO use Swagger instead, this is silly
type Project struct {
	Id              int    `json:"id"`
	Label           string `json:"label"`
	Description     string `json:"description"`
	Repository      string `json:"repository"`
	OrgSlug         string `json:"org_slug"`
	LocalRepoRemote string `json:"local_repo_remote"`
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
