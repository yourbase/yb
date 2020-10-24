package buildpack

import (
	"context"
	"fmt"

	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/plumbing"
	"zombiezen.com/go/log"
)

const androidNDKDistMirror = "https://dl.google.com/android/repository/android-ndk-{{.Version}}-{{.OS}}-{{.Arch}}.zip"

func installAndroidNDK(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (biome.Environment, error) {
	ndkDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "android-ndk", "android-ndk-"+spec.Version())
	env := biome.Environment{
		Vars: map[string]string{
			"ANDROID_NDK_HOME": ndkDir,
		},
	}

	if _, err := biome.EvalSymlinks(ctx, sys.Biome, ndkDir); err == nil {
		log.Infof(ctx, "Found Android NDK at %s", ndkDir)
		return env, nil
	}

	desc := sys.Biome.Describe()
	data := struct {
		OS      string
		Arch    string
		Version string
	}{
		map[string]string{
			biome.Linux: "linux",
			biome.MacOS: "darwin",
		}[desc.OS],
		map[string]string{
			biome.Intel64: "x86_64",
		}[desc.Arch],
		spec.Version(),
	}
	if data.OS == "" || data.Arch == "" {
		return biome.Environment{}, fmt.Errorf("unsupported os/arch %s/%s", desc.OS, desc.Arch)
	}
	downloadURL, err := plumbing.TemplateToString(androidNDKDistMirror, data)
	if err != nil {
		return biome.Environment{}, err
	}

	log.Infof(ctx, "Installing Android NDK v%s in %s...", spec.Version(), ndkDir)
	if err := extract(ctx, sys, ndkDir, downloadURL, stripTopDirectory); err != nil {
		return biome.Environment{}, err
	}
	return env, nil
}
