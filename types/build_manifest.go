package types

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
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
	err = b.mergeBuildDependencies(targetList)
	if err != nil {
		return targetList, err
	}

	return targetList, nil
}

// mergeBuildDependencies will override and also merge build packs defined for each BuildTarget when
// resolving which ones should be built in ResolveBuildTargets above. It does so by checking if the
// Manifest already defined the same `tool:version` as the one described inside the build target
// TODO use cue instead, to replace all this error prone behavior
func (b BuildManifest) mergeBuildDependencies(targetList []BuildTarget) error {
	manifestSortedBuildDepList := b.Dependencies.Build
	sort.Strings(manifestSortedBuildDepList)
	for _, buildTarget := range targetList {
		for _, dep := range buildTarget.Dependencies.Build {
			for i, globalDep := range manifestSortedBuildDepList {
				// Extract tool prefix
				parts := strings.Split(dep, ":")
				if len(parts) != 2 {
					return fmt.Errorf("merging build deps: malformed build pack definition: %s", dep)
				}
				toolPrefix := parts[0]
				if strings.HasPrefix(globalDep, toolPrefix) {
					manifestSortedBuildDepList[i] = dep // override by replacing e.g. `flutter:xxx` with `flutter:yyy`
				}
			}
			if i := sort.SearchStrings(manifestSortedBuildDepList, dep); i == len(manifestSortedBuildDepList) { // dep wasn't found
				// merge
				manifestSortedBuildDepList = append(manifestSortedBuildDepList, dep)
			}
		}
	}
	return nil
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
