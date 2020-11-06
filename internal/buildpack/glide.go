package buildpack

import (
	"context"

	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log"
)

func installGlide(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (biome.Environment, error) {
	glideDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "glide-"+spec.Version())
	env := biome.Environment{
		PrependPath: []string{
			sys.Biome.JoinPath(glideDir),
		},
	}

	// If directory already exists, then use it.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, glideDir); err == nil {
		log.Infof(ctx, "Ant v%s located in %s", spec.Version(), glideDir)
		return env, nil
	}

	log.Infof(ctx, "Installing Glide v%s in %s", spec.Version(), glideDir)
	const template = "https://github.com/Masterminds/glide/releases/download/v{{.Version}}/glide-v{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz"
	desc := sys.Biome.Describe()
	data := struct {
		Version string
		OS      string
		Arch    string
	}{
		Version: spec.Version(),
		OS:      desc.OS,
		Arch:    desc.Arch,
	}
	downloadURL, err := templateToString(template, data)
	if err != nil {
		return biome.Environment{}, err
	}
	if err := extract(ctx, sys, glideDir, downloadURL, stripTopDirectory); err != nil {
		return biome.Environment{}, err
	}
	return env, nil
}
