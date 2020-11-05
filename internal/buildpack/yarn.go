package buildpack

import (
	"context"
	"fmt"

	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log"
)

func installYarn(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (biome.Environment, error) {
	yarnDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "yarn", "yarn-v"+spec.Version())
	env := biome.Environment{
		PrependPath: []string{
			sys.Biome.JoinPath(yarnDir, "bin"),
		},
	}

	// If directory already exists, then use it.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, yarnDir); err == nil {
		log.Infof(ctx, "Yarn v%s located in %s", spec.Version(), yarnDir)
		return env, nil
	}

	log.Infof(ctx, "Installing Yarn v%s in %s", spec.Version(), yarnDir)
	downloadURL := fmt.Sprintf("https://github.com/yarnpkg/yarn/releases/download/v%s/yarn-v%s.tar.gz", spec.Version(), spec.Version())
	if err := extract(ctx, sys, yarnDir, downloadURL, stripTopDirectory); err != nil {
		return biome.Environment{}, err
	}
	return env, nil
}
