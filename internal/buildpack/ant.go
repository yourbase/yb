package buildpack

import (
	"context"
	"fmt"

	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/types"
	"zombiezen.com/go/log"
)

func installAnt(ctx context.Context, sys Sys, spec types.BuildpackSpec) (biome.Environment, error) {
	antDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "ant", "apache-ant-"+spec.Version())
	env := biome.Environment{
		PrependPath: []string{
			sys.Biome.JoinPath(antDir, "bin"),
		},
	}

	// If directory already exists, then use it.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, antDir); err == nil {
		log.Infof(ctx, "Ant v%s located in %s", spec.Version(), antDir)
		return env, nil
	}

	log.Infof(ctx, "Installing Ant v%s in %s", spec.Version(), antDir)
	downloadURL := fmt.Sprintf("https://mirrors.sonic.net/apache/ant/binaries/apache-ant-%s-bin.zip", spec.Version())
	if err := extract(ctx, sys, antDir, downloadURL, stripTopDirectory); err != nil {
		return biome.Environment{}, err
	}
	return env, nil
}
