package buildpack

import (
	"context"
	"fmt"

	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log"
)

func installHeroku(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (biome.Environment, error) {
	if spec.Version() != "latest" {
		return biome.Environment{}, fmt.Errorf("'latest' is the only allowed version")
	}
	herokuDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "heroku")
	env := biome.Environment{
		PrependPath: []string{
			sys.Biome.JoinPath(herokuDir, "bin"),
		},
	}

	if _, err := biome.EvalSymlinks(ctx, sys.Biome, herokuDir); err == nil {
		// Directory already exists: update it.
		log.Infof(ctx, "Heroku located in %s; running update...", herokuDir)
		err := sys.Biome.Run(ctx, &biome.Invocation{
			Argv:   []string{"heroku", "update"},
			Env:    env,
			Stdout: sys.Stdout,
			Stderr: sys.Stderr,
		})
		if err != nil {
			return biome.Environment{}, err
		}
	} else {
		// Download Heroku.
		log.Infof(ctx, "Installing Heroku in %s", herokuDir)
		const template = "https://cli-assets.heroku.com/heroku-{{.OS}}-{{.Arch}}.tar.gz"
		desc := sys.Biome.Describe()
		data := struct {
			OS   string
			Arch string
		}{
			OS: map[string]string{
				biome.Linux: "linux",
				biome.MacOS: "darwin",
			}[desc.OS],
			Arch: map[string]string{
				biome.Intel64: "x64",
				biome.Intel32: "x86",
			}[desc.Arch],
		}
		if data.OS == "" {
			return biome.Environment{}, fmt.Errorf("unsupported os %s", desc.OS)
		}
		if data.Arch == "" {
			return biome.Environment{}, fmt.Errorf("unsupported architecture %s", desc.Arch)
		}
		downloadURL, err := templateToString(template, data)
		if err != nil {
			return biome.Environment{}, err
		}
		if err := extract(ctx, sys, herokuDir, downloadURL, stripTopDirectory); err != nil {
			return biome.Environment{}, err
		}
	}
	return env, nil
}
