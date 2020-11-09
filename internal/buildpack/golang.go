package buildpack

import (
	"context"
	"fmt"

	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log"
)

func installGo(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (biome.Environment, error) {
	golangRoot := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "go")
	golangDir := sys.Biome.JoinPath(golangRoot, "go"+spec.Version())
	gopathDir := sys.Biome.JoinPath(golangRoot, "gopath")
	env := biome.Environment{
		Vars: map[string]string{
			"GOROOT": golangDir,
			"GOPATH": gopathDir + ":" + sys.Biome.Dirs().Package,
		},
		PrependPath: []string{
			gopathDir,
			sys.Biome.JoinPath(golangDir, "bin"),
		},
	}

	// If directory already exists, then use it.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, golangDir); err == nil {
		log.Infof(ctx, "Go v%s located in %s", spec.Version(), golangDir)
		return env, nil
	}

	log.Infof(ctx, "Installing Go v%s in %s", spec.Version(), golangDir)
	desc := sys.Biome.Describe()
	downloadURL := fmt.Sprintf("https://dl.google.com/go/go%s.%s-%s.tar.gz", spec.Version(), desc.OS, desc.Arch)
	if err := extract(ctx, sys, golangDir, downloadURL, stripTopDirectory); err != nil {
		return biome.Environment{}, err
	}
	return env, nil
}
