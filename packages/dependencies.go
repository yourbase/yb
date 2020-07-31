package packages

import (
	"fmt"
	"strings"

	"github.com/yourbase/yb/types"
)

// mergeDeps overrides and merge dependencies defined per build target,
// adding globally defined deps to per-build target defined dependencies,
// where it wasn't added
func mergeDeps(b *types.BuildManifest) error {
	splitToolName := func(dep string) (tool, version string, _ error) {
		parts := strings.SplitN(dep, ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("merging/overriding build localDeps: malformed build pack definition: %s", dep)
		}
		tool = parts[0]
		version = parts[1]
		return
	}
	globalDepsMap := make(map[string]string)
	buildTgtsDepMap := make(map[string]map[string]string)
	globalDeps := b.Dependencies.Build
	targetList := b.BuildTargets

	for _, dep := range globalDeps {
		tool, version, err := splitToolName(dep)
		if err != nil {
			return err
		}
		globalDepsMap[tool] = version
	}
	for _, tgt := range targetList {
		tgtToolMap := make(map[string]string)
		for _, dep := range tgt.Dependencies.Build {
			tool, version, err := splitToolName(dep)
			if err != nil {
				return err
			}
			tgtToolMap[tool] = version
		}
		buildTgtsDepMap[tgt.Name] = tgtToolMap
	}
	for k, v := range globalDepsMap {
		for t, m := range buildTgtsDepMap {
			tgt, err := b.BuildTarget(t)
			if err != nil {
				return err
			}
			if _, exists := m[k]; !exists {
				// Add all global build deps that isn't set for each build target
				deps := tgt.Dependencies.Build
				deps = append(deps, k+":"+v)
			}
		}
	}

	return nil
}
