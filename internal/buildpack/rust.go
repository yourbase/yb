package buildpack

import (
	"context"
	"fmt"

	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log"
)

func installRust(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (biome.Environment, error) {
	rustDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "rust", "rust-"+spec.Version())
	rustDownloadDir := rustDir + "-download"
	env := biome.Environment{
		Vars: map[string]string{
			"CARGO_HOME": sys.Biome.JoinPath(sys.Biome.Dirs().Home, "cargohome"),
		},
		PrependPath: []string{
			sys.Biome.JoinPath(rustDir, "bin"),
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
	downloadURL, err := templateToString(template, data)
	if err != nil {
		return biome.Environment{}, err
	}
	if err := extract(ctx, sys, rustDownloadDir, downloadURL, stripTopDirectory); err != nil {
		return biome.Environment{}, err
	}
	err = sys.Biome.Run(ctx, &biome.Invocation{
		Argv: []string{
			sys.Biome.JoinPath(rustDownloadDir, "install.sh"),
			"--prefix=" + rustDir,
		},
		Dir:    rustDownloadDir,
		Stdout: sys.Stdout,
		Stderr: sys.Stderr,
	})
	if err != nil {
		return biome.Environment{}, err
	}
	return env, nil
}
