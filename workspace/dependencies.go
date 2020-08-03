package workspace

import (
	"fmt"
	"strings"
)

func (bt BuildTarget) mergeDeps(globalDepsList []string) ([]string, error) {
	if len(bt.Dependencies.Build) == 0 {
		return globalDepsList, nil
	}
	splitToolName := func(dep string) (tool, version string, _ error) {
		parts := strings.SplitN(dep, ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("merging/overriding build localDeps: malformed build pack definition: %s", dep)
		}
		tool = parts[0]
		version = parts[1]
		return
	}

	if len(globalDepsList) == 0 {
		return bt.Dependencies.Build, nil
	}

	globalDepsMap := make(map[string]string)
	for _, dep := range globalDepsList {
		tool, version, err := splitToolName(dep)
		if err != nil {
			return nil, err
		}
		globalDepsMap[tool] = version
	}
	buildTgtDepsMap := make(map[string]string)
	for _, dep := range bt.Dependencies.Build {
		tool, version, err := splitToolName(dep)
		if err != nil {
			return nil, err
		}
		buildTgtDepsMap[tool] = version
	}

	finalDepsList := make([]string, 0)
	finalDepsList = append(finalDepsList, bt.Dependencies.Build...)

	for k, v := range globalDepsMap {
		if _, exists := buildTgtDepsMap[k]; !exists {
			finalDepsList = append(finalDepsList, k+":"+v)
		}
	}

	return finalDepsList, nil
}
