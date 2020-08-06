package workspace

import (
	"fmt"
	"strings"
)

func (bt *BuildTarget) mergeDeps(globalDepsList []string) error {
	globalDepsMap := make(map[string]string)
	for _, dep := range globalDepsList {
		tool, version, err := SplitToolSpec(dep)
		if err != nil {
			return fmt.Errorf("merging/overriding build localDeps: %w", err)
		}
		globalDepsMap[tool] = version
	}
	buildTgtDepsMap := make(map[string]string)
	for _, dep := range bt.Dependencies.Build {
		for tool, version := range globalDepsMap {
			buildTgtDepsMap[tool] = version
		}
		tool, version, err := SplitToolSpec(dep)
		if err != nil {
			return fmt.Errorf("merging/overriding build localDeps: %w", err)
		}
		buildTgtDepsMap[tool] = version
	}

	bt.Dependencies.Build = bt.Dependencies.Build[:0]

	for tool, version := range buildTgtDepsMap {
		bt.Dependencies.Build = append(bt.Dependencies.Build, tool+":"+version)
	}

	return nil
}

// SplitToolSpec gives tool name and version from a dependency spec
func SplitToolSpec(dep string) (tool, version string, _ error) {
	parts := strings.SplitN(dep, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("malformed build pack definition: %s", dep)
	}
	tool = parts[0]
	version = parts[1]
	return
}
