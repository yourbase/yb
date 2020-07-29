package types

import (
	"fmt"
	"strings"
)

func (b BuildManifest) FinalBuildDependencies(targetList []BuildTarget) ([]string, error) {
	buildDeps, err := b.mergeDeps(targetList)
	if err != nil {
		return nil, err
	}
	if len(buildDeps) > 0 {
		return buildDeps, nil
	}
	return b.Dependencies.Build, nil
}

// mergeDeps overrides and merge dependencies defined per build target,
// effectively swaping BuildManifest.Dependencies.Build []string with a new list
func (b BuildManifest) mergeDeps(targetList []BuildTarget) ([]string, error) {
	splitToolName := func(dep string) (tool, version string, _ error) {
		parts := strings.SplitN(dep, ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("merging/overriding build localDeps: malformed build pack definition: %s", dep)
		}
		tool = parts[0]
		version = parts[1]
		return
	}
	depMap := make(map[string]string)
	globalDeps := b.Dependencies.Build

	for _, dep := range globalDeps {
		tool, version, err := splitToolName(dep)
		if err != nil {
			return nil, err
		}
		depMap[tool] = version
	}
	// Doing this before makes locally defined deps to override the global definition
	for _, tgt := range targetList {
		for _, dep := range tgt.Dependencies.Build {
			tool, version, err := splitToolName(dep)
			if err != nil {
				return nil, err
			}
			depMap[tool] = version
		}
	}
	depList := make([]string, 0)
	for k, v := range depMap {
		toolSpec := k + ":" + v
		depList = append(depList, toolSpec)
	}

	return depList, nil
}
