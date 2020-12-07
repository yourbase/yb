package buildpack

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yourbase/commons/xcontext"
	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log"
)

func installR(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (_ biome.Environment, err error) {
	version := spec.Version()
	dotIndex := strings.IndexByte(version, '.')
	if dotIndex == -1 {
		return biome.Environment{}, fmt.Errorf("invalid version %q", version)
	}
	majorVersion := version[:dotIndex]

	rlangDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "R", "R-"+version)
	env := biome.Environment{
		PrependPath: []string{
			sys.Biome.JoinPath(rlangDir, "bin"),
		},
	}

	// If directory already exists, then use it.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, rlangDir); err == nil {
		log.Infof(ctx, "R v%s located in %s", version, rlangDir)
		return env, nil
	}

	srcDir := sys.Biome.JoinPath(rlangDir, "src")
	log.Infof(ctx, "Downloading R v%s to %s...", version, srcDir)
	const template = "https://cloud.r-project.org/src/base/R-{{.MajorVersion}}/R-{{.Version}}.tar.gz"
	data := struct {
		Version      string
		MajorVersion string
	}{
		Version:      version,
		MajorVersion: majorVersion,
	}
	downloadURL, err := templateToString(template, data)
	if err != nil {
		return biome.Environment{}, err
	}
	if err := extract(ctx, sys, srcDir, downloadURL, stripTopDirectory); err != nil {
		return biome.Environment{}, err
	}
	defer func() {
		// Remove build directory on failure to prevent subsequent invocations from
		// using a corrupt installation.
		if err != nil {
			rmCtx, cancel := xcontext.KeepAlive(ctx, 10*time.Second)
			defer cancel()
			rmErr := sys.Biome.Run(rmCtx, &biome.Invocation{
				Argv:   []string{"rm", "-rf", rlangDir},
				Stdout: sys.Stdout,
				Stderr: sys.Stderr,
			})
			if rmErr != nil {
				log.Warnf(ctx, "Cleaning up failed R install: %v", rmErr)
			}
		}
	}()

	log.Infof(ctx, "Compiling R v%s in %s...", version, srcDir)
	commands := [][]string{
		{
			sys.Biome.JoinPath(srcDir, "configure"),
			"--with-x=no",
			"--prefix=" + rlangDir,
		},
		{"make", "--jobs=2"},
		{"make", "install"},
	}
	for _, argv := range commands {
		err := sys.Biome.Run(ctx, &biome.Invocation{
			Argv:   argv,
			Dir:    srcDir,
			Stdout: sys.Stdout,
			Stderr: sys.Stderr,
		})
		if err != nil {
			return biome.Environment{}, fmt.Errorf("compiling R: %s: %w", argv[0], err)
		}
	}
	return env, nil
}
