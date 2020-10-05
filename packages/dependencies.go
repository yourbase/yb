package packages

import (
	"fmt"

	"github.com/yourbase/yb/buildpacks"
	"github.com/yourbase/yb/types"
)

// mergeDeps overrides and merge build dependencies into
// the BuildTarget.Dependencies.Build field. Adding globally defined deps to
// per-build target defined dependencies, where it wasn't added.
func mergeDeps(b *types.BuildManifest) error {
	globalDepsMap := make(map[string]string) // tool -> version
	globalDeps := b.Dependencies.Build
	targetList := b.BuildTargets

	for _, dep := range globalDeps {
		tool, version, err := buildpacks.SplitToolSpec(dep)
		if err != nil {
			return fmt.Errorf("merging/overriding build localDeps: %w", err)
		}
		globalDepsMap[tool] = version
	}
	for _, tgt := range targetList {
		tgtToolMap := make(map[string]string)
		for tool, version := range globalDepsMap {
			tgtToolMap[tool] = version
		}
		for _, dep := range tgt.Dependencies.Build {
			tool, version, err := buildpacks.SplitToolSpec(dep)
			if err != nil {
				return fmt.Errorf("merging/overriding build localDeps: %w", err)
			}
			tgtToolMap[tool] = version
		}
		tgt.Dependencies.Build = tgt.Dependencies.Build[:0]
		for tool, version := range tgtToolMap {
			tgt.Dependencies.Build = append(tgt.Dependencies.Build, tool+":"+version)
		}
	}
	return nil
}
