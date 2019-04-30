package types

import (
	"fmt"
)

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
