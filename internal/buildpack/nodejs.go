package buildpack

import (
	"context"
	"fmt"

	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/types"
	"zombiezen.com/go/log"
)

func installNode(ctx context.Context, sys Sys, spec types.BuildpackSpec) (biome.Environment, error) {
	contextDirs := sys.Biome.Dirs()
	nodeDir := sys.Biome.JoinPath(contextDirs.Tools, "nodejs", "node-"+spec.Version())
	env := biome.Environment{
		Vars: map[string]string{
			// TODO: Fix this to be the package cache?
			"NODE_PATH": contextDirs.Package,
		},
		PrependPath: []string{
			sys.Biome.JoinPath(contextDirs.Package, "node_modules", ".bin"),
			sys.Biome.JoinPath(nodeDir, "bin"),
		},
	}

	// If directory already exists, then use it.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, nodeDir); err == nil {
		log.Infof(ctx, "Node v%s located in %s", spec.Version(), nodeDir)
		return env, nil
	}

	log.Infof(ctx, "Installing Node v%s in %s", spec.Version(), nodeDir)
	const template = "https://nodejs.org/dist/v{{.Version}}/node-v{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz"
	desc := sys.Biome.Describe()
	data := struct {
		Version string
		OS      string
		Arch    string
	}{
		spec.Version(),
		map[string]string{
			biome.Linux: "linux",
			biome.MacOS: "darwin",
		}[desc.OS],
		map[string]string{
			biome.Intel64: "x64",
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
	if err := extract(ctx, sys, nodeDir, downloadURL, stripTopDirectory); err != nil {
		return biome.Environment{}, err
	}
	return env, nil
}
