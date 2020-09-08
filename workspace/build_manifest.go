package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/yourbase/narwhal"
	"github.com/yourbase/yb/runtime"
)

const DependencyChecksumLength = 12

type CIInfo struct {
	CIBuilds []CIBuild `yaml:"builds"`
}

type CIBuild struct {
	Name        string `yaml:"name"`
	BuildTarget string `yaml:"build_target"`
	When        string `yaml:"when"`
}

type PackagePhase struct {
	Artifacts []string          `yaml:"artifacts"`
	Docker    DockerArchive     `yaml:"docker"`
	Tar       map[string]string `yaml:"tar"`
}

type DockerArchive struct {
	BaseImage  string `yaml:"base_image"`
	WorkingDir string `yaml:"working_dir"`
	Exec       string `yaml:"exec"`
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

// environmentVariables returns a slice of parsed environment variables for this ExecPhase
func (e *ExecPhase) environmentVariables(ctx context.Context, envName string, data runtime.RuntimeEnvironmentData) ([]string, error) {
	packs := make([][]string, 0)
	packs = append(packs, e.Environment["default"])

	fromContainers := make([]string, 0)
	for k, v := range data.Containers.Environment(ctx) {
		fromContainers = append(fromContainers, k+"="+v)
	}
	packs = append(packs, fromContainers)

	if envName != "default" {
		packs = append(packs, e.Environment[envName])
	}

	return parseEnvironment(ctx, ".env", data, packs...)
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

type BuildManifest struct {
	Dependencies DependencySet `yaml:"dependencies"`
	Sandbox      bool          `yaml:"sandbox"`
	BuildTargets []BuildTarget `yaml:"build_targets"`
	Build        BuildTarget   `yaml:"build"`
	Exec         ExecPhase     `yaml:"exec"`
	Package      PackagePhase  `yaml:"package"`
	CI           CIInfo        `yaml:"ci"`
}

func (b BuildManifest) BuildDependenciesChecksum() string {
	buf := bytes.Buffer{}
	for _, dep := range b.Dependencies.Build {
		buf.Write([]byte(dep))
	}

	sum := sha256.Sum256(buf.Bytes())
	return fmt.Sprintf("%x", sum[:DependencyChecksumLength])
}

func (b BuildManifest) IsTargetSandboxed(target BuildTarget) bool {
	return b.Sandbox || target.Sandbox
}

// XXX: Support more than one level? Intuitively that seems like it will breed un-needed complexity
func (b BuildManifest) ResolveBuildTargets(targetName string) ([]BuildTarget, error) {
	targetList := make([]BuildTarget, 0)

	target, err := b.BuildTarget(targetName)
	if err != nil {
		return targetList, err
	}

	if len(target.BuildAfter) > 0 {
		for _, depName := range target.BuildAfter {
			depTarget, err := b.BuildTarget(depName)
			if err != nil {
				return targetList, err
			}
			targetList = append(targetList, depTarget)
		}
	}

	targetList = append(targetList, target)

	return targetList, nil
}

func (b BuildManifest) CIBuild(buildName string) (CIBuild, error) {
	for _, build := range b.CI.CIBuilds {
		if build.Name == buildName {
			return build, nil
		}
	}
	return CIBuild{}, fmt.Errorf("no such CI build '%s' in build manifest", buildName)
}

func (b BuildManifest) BuildTarget(targetName string) (BuildTarget, error) {
	for _, target := range b.BuildTargets {
		if target.Name == targetName {
			return target, nil
		}
	}
	return BuildTarget{}, fmt.Errorf("no such target '%s' in build manifest", targetName)
}

func (b BuildManifest) BuildTargetList() []string {
	targets := make([]string, 0, len(b.BuildTargets))

	for _, t := range b.BuildTargets {
		targets = append(targets, t.Name)
	}

	return targets
}
