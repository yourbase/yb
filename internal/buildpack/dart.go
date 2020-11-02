package buildpack

import (
	"context"
	"fmt"

	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/types"
	"zombiezen.com/go/log"
)

func installDart(ctx context.Context, sys Sys, spec types.BuildpackSpec) (biome.Environment, error) {
	dartDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "dart", "dart-sdk-"+spec.Version())
	env := biome.Environment{
		PrependPath: []string{
			sys.Biome.JoinPath(dartDir, "bin"),
		},
	}

	// If directory already exists, then use it.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, dartDir); err == nil {
		log.Infof(ctx, "Dart v%s located in %s", spec.Version(), dartDir)
		return env, nil
	}

	log.Infof(ctx, "Installing Dart v%s in %s", spec.Version(), dartDir)
	desc := sys.Biome.Describe()
	const template = "https://storage.googleapis.com/dart-archive/channels/stable/release/{{.Version}}/sdk/dartsdk-{{.OS}}-{{.Arch}}-release.zip"
	data := struct {
		Version string
		OS      string
		Arch    string
	}{
		spec.Version(),
		map[string]string{
			biome.Linux: "linux",
			biome.MacOS: "macos",
		}[desc.OS],
		map[string]string{
			biome.Intel64: "x64",
		}[desc.Arch],
	}
	if data.OS == "" {
		return biome.Environment{}, fmt.Errorf("unsupported os %s", desc.OS)
	}
	if data.OS == "" {
		return biome.Environment{}, fmt.Errorf("unsupported architecture %s", desc.Arch)
	}
	downloadURL, err := plumbing.TemplateToString(template, data)
	if err != nil {
		return biome.Environment{}, err
	}
	if err := extract(ctx, sys, dartDir, downloadURL, stripTopDirectory); err != nil {
		return biome.Environment{}, err
	}
	return env, nil
}
