package types

import (
	"bytes"
	"crypto/sha256"
	"fmt"
)

const DependencyChecksumLength = 12

func (b BuildManifest) BuildDependenciesChecksum() string {
	buf := bytes.Buffer{}
	for _, dep := range b.Dependencies.Build {
		buf.Write([]byte(dep))
	}

	sum := sha256.Sum256(buf.Bytes())
	return fmt.Sprintf("%x", sum[:DependencyChecksumLength])
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
	return CIBuild{}, fmt.Errorf("No such CI build '%s' in build manifest", buildName)
}

func (b BuildManifest) BuildTarget(targetName string) (BuildTarget, error) {
	for _, target := range b.BuildTargets {
		if target.Name == targetName {
			return target, nil
		}
	}
	return BuildTarget{}, fmt.Errorf("No such target '%s' in build manifest", targetName)
}

func (b BuildManifest) BuildTargetList() []string {
	targets := make([]string, 0, len(b.BuildTargets))

	for _, t := range b.BuildTargets {
		targets = append(targets, t.Name)
	}

	return targets
}
