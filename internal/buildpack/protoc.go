package buildpack

import (
	"context"
	"fmt"

	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/plumbing"
	"zombiezen.com/go/log"
)

func installProtoc(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (biome.Environment, error) {
	protocDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "protoc", "protoc-"+spec.Version())
	env := biome.Environment{
		PrependPath: []string{
			sys.Biome.JoinPath(protocDir, "bin"),
		},
	}

	// If directory already exists, then use it.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, protocDir); err == nil {
		log.Infof(ctx, "protoc v%s located in %s", spec.Version(), protocDir)
		return env, nil
	}

	log.Infof(ctx, "Installing protoc v%s in %s", spec.Version(), protocDir)
	const template = "https://github.com/google/protobuf/releases/download/v{{.Version}}/protoc-{{.Version}}-{{.Variant}}.zip"
	data := struct {
		Version string
		Variant string
	}{
		Version: spec.Version(),
	}
	switch desc := sys.Biome.Describe(); {
	case desc.OS == biome.Linux && desc.Arch == biome.Intel64:
		data.Variant = "linux-x86_64"
	case desc.OS == biome.Linux && desc.Arch == biome.Intel32:
		data.Variant = "linux-x86_32"
	case desc.OS == biome.MacOS && desc.Arch == biome.Intel64:
		data.Variant = "osx-x86_64"
	case desc.OS == biome.Windows && desc.Arch == biome.Intel64:
		data.Variant = "win64"
	case desc.OS == biome.Windows && desc.Arch == biome.Intel32:
		data.Variant = "win32"
	default:
		return biome.Environment{}, fmt.Errorf("unsupported os/arch %s/%s", desc.OS, desc.Arch)
	}
	downloadURL, err := plumbing.TemplateToString(template, data)
	if err != nil {
		return biome.Environment{}, err
	}
	if err := extract(ctx, sys, protocDir, downloadURL, tarbomb); err != nil {
		return biome.Environment{}, err
	}
	return env, nil
}
