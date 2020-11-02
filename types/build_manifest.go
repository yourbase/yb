package types

import (
	"bytes"
	"crypto/sha256"
	"fmt"
)

type BuildManifest struct {
	Dependencies DependencySet  `yaml:"dependencies"`
	Sandbox      bool           `yaml:"sandbox"`
	BuildTargets []*BuildTarget `yaml:"build_targets"`
	Build        *BuildTarget   `yaml:"build"`
	Exec         *ExecPhase     `yaml:"exec"`
	Package      *PackagePhase  `yaml:"package"`
	CI           *CIInfo        `yaml:"ci"`
}

const dependencyChecksumLength = 12

func (b BuildManifest) BuildDependenciesChecksum() string {
	buf := bytes.Buffer{}
	for _, dep := range b.Dependencies.Build {
		buf.Write([]byte(dep))
	}

	sum := sha256.Sum256(buf.Bytes())
	return fmt.Sprintf("%x", sum[:dependencyChecksumLength])
}

// BuildOrder returns a topological sort of the targets needed to build the
// given target. On success, the last element in the slice is always the target
// with the given name.
func (b BuildManifest) BuildOrder(targetName string) ([]*BuildTarget, error) {
	target, err := b.BuildTarget(targetName)
	if err != nil {
		return nil, fmt.Errorf("determine build order for %s: %w", targetName, err)
	}

	// TODO(ch2750): This only handles direct dependencies.
	var targetList []*BuildTarget
	for _, depName := range target.BuildAfter {
		depTarget, err := b.BuildTarget(depName)
		if err != nil {
			return nil, fmt.Errorf("determine build order for %s: %w", targetName, err)
		}
		targetList = append(targetList, depTarget)
	}
	targetList = append(targetList, target)
	return targetList, nil
}

func (b BuildManifest) CIBuild(buildName string) (*CIBuild, error) {
	for _, build := range b.CI.CIBuilds {
		if build.Name == buildName {
			return build, nil
		}
	}
	return nil, fmt.Errorf("no such CI build '%s' in build manifest", buildName)
}

func (b BuildManifest) BuildTarget(targetName string) (*BuildTarget, error) {
	for _, target := range b.BuildTargets {
		if target.Name == targetName {
			return target, nil
		}
	}
	return nil, fmt.Errorf("%s: no such target", targetName)
}

func (b BuildManifest) BuildTargetList() []string {
	targets := make([]string, 0, len(b.BuildTargets))

	for _, t := range b.BuildTargets {
		targets = append(targets, t.Name)
	}

	return targets
}
