package buildpack

import (
	"context"
	"fmt"

	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/types"
	"zombiezen.com/go/log"
)

func installRust(ctx context.Context, sys Sys, spec types.BuildpackSpec) (biome.Environment, error) {
	rustDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "rust", "rust-"+spec.Version())
	env := biome.Environment{
		Vars: map[string]string{
			"CARGO_HOME": sys.Biome.JoinPath(sys.Biome.Dirs().Home, "cargohome"),
		},
		PrependPath: []string{
			sys.Biome.JoinPath(rustDir, "cargo", "bin"),
			sys.Biome.JoinPath(rustDir, "rustc", "bin"),
		},
	}

	// If directory already exists, then use it.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, rustDir); err == nil {
		log.Infof(ctx, "Rust v%s located in %s", spec.Version(), rustDir)
		return env, nil
	}

	log.Infof(ctx, "Installing Rust v%s in %s", spec.Version(), rustDir)
	const template = "https://static.rust-lang.org/dist/rust-{{.Version}}-{{.Arch}}-{{.OS}}.tar.gz"
	desc := sys.Biome.Describe()
	data := struct {
		Version string
		OS      string
		Arch    string
	}{
		Version: spec.Version(),
		OS: map[string]string{
			biome.Linux: "unknown-linux-gnu",
			biome.MacOS: "apple-darwin",
		}[desc.OS],
		Arch: map[string]string{
			biome.Intel64: "x86_64",
			biome.Intel32: "i686",
		}[desc.Arch],
	}
	if data.OS == "" {
		return biome.Environment{}, fmt.Errorf("unsupported os %s", desc.OS)
	}
	if data.Arch == "" {
		return biome.Environment{}, fmt.Errorf("unsupported architecture %s", desc.Arch)
	}
	downloadURL, err := plumbing.TemplateToString(template, data)
	if err != nil {
		return biome.Environment{}, err
	}
	if err := extract(ctx, sys, rustDir, downloadURL, stripTopDirectory); err != nil {
		return biome.Environment{}, err
	}
	return env, nil
}
