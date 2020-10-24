package buildpack

import (
	"context"
	"fmt"
	"strings"

	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/plumbing"
	"github.com/yourbase/yb/internal/ybdata"
	"zombiezen.com/go/log"
)

// TODO: Install libssl-dev (or equivalent / warn) and zlib-dev based on platform

func installRuby(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (biome.Environment, error) {
	rbenvDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "rbenv")
	rubyDir := sys.Biome.JoinPath(rbenvDir, "versions", spec.Version())
	gemsDir := sys.Biome.JoinPath(sys.Biome.Dirs().Home, "rubygems")
	env := biome.Environment{
		Vars: map[string]string{
			"GEM_HOME": gemsDir,
		},
		PrependPath: []string{
			sys.Biome.JoinPath(gemsDir, "bin"),
			sys.Biome.JoinPath(rubyDir, "bin"),
		},
	}

	// If directory already exists, then use it.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, rubyDir); err == nil {
		log.Infof(ctx, "Ruby v%s located in %s", spec.Version(), rubyDir)
		return env, nil
	}

	// Try to download a YourBase pre-built binary.
	desc := sys.Biome.Describe()
	const template = "https://yourbase-build-tools.s3-us-west-2.amazonaws.com/ruby/ruby-{{ .Version }}-{{ .OS }}-{{ .Arch }}-{{ .DistroCodename }}.tar.bz2"
	data := struct {
		Version        string
		OS             string
		DistroCodename string
		Arch           string
	}{
		Version: spec.Version(),
		OS: map[string]string{
			biome.Linux: "Linux",
		}[desc.OS],
		Arch: map[string]string{
			biome.Intel64: "x86_64",
		}[desc.Arch],
	}
	if data.OS == "" || data.Arch == "" {
		log.Debugf(ctx, "Pre-built binaries unsupported for %s/%s", desc.OS, desc.Arch)
	} else {
		log.Infof(ctx, "Searching for YourBase-built Ruby binary...")
		var err error
		data.DistroCodename, err = readLSBCodename(ctx, sys)
		if err != nil {
			log.Warnf(ctx, "Skipping search for pre-built binary: %v", err)
		} else {
			downloadURL, err := plumbing.TemplateToString(template, data)
			if err != nil {
				return biome.Environment{}, fmt.Errorf("download pre-built binary: %w", err)
			}
			err = extract(ctx, sys, rubyDir, downloadURL, stripTopDirectory)
			if err == nil {
				return env, nil
			}
			if !ybdata.IsNotFound(err) {
				return biome.Environment{}, fmt.Errorf("download pre-built binary: %w", err)
			}
		}
	}

	// Use rbenv to install Ruby.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, rbenvDir); err != nil {
		log.Debugf(ctx, "rbenv not found: %v", err)
		log.Infof(ctx, "Installing rbenv in %s", rbenvDir)
		err := extract(ctx, sys, rbenvDir, "https://github.com/rbenv/rbenv/archive/master.zip", stripTopDirectory)
		if err != nil {
			return biome.Environment{}, fmt.Errorf("download rbenv: %w", err)
		}
	}
	rubyBuildDir := sys.Biome.JoinPath(rbenvDir, "plugins", "ruby-build")
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, rubyBuildDir); err != nil {
		log.Debugf(ctx, "ruby-build not found: %v", err)
		log.Infof(ctx, "Installing ruby-build plugin in %s", rubyBuildDir)
		err := extract(ctx, sys, rubyBuildDir, "https://github.com/rbenv/ruby-build/archive/master.zip", stripTopDirectory)
		if err != nil {
			return biome.Environment{}, fmt.Errorf("download ruby-build plugin: %w", err)
		}
	}
	err := sys.Biome.Run(ctx, &biome.Invocation{
		Argv: []string{"rbenv", "install", spec.Version()},
		Env: biome.Environment{
			Vars: map[string]string{"RBENV_ROOT": rbenvDir},
			PrependPath: []string{
				sys.Biome.JoinPath(rbenvDir, "bin"),
			},
		},
		Stdout: sys.Stdout,
		Stderr: sys.Stderr,
	})
	if err != nil {
		return biome.Environment{}, fmt.Errorf("rbenv: %w", err)
	}
	return env, nil
}

func readLSBCodename(ctx context.Context, sys Sys) (string, error) {
	const filename = "/etc/os-release"
	const varname = "VERSION_CODENAME"
	info := new(strings.Builder)
	err := sys.Biome.Run(ctx, &biome.Invocation{
		Argv:   []string{"cat", filename},
		Stdout: info,
		Stderr: sys.Stderr,
	})
	if err != nil {
		return "", fmt.Errorf("read lsb release: %w", err)
	}
	lines := strings.Split(info.String(), "\n")
	const varPrefix = varname + "="
	for _, line := range lines {
		if strings.HasPrefix(line, varPrefix) {
			return line[len(varPrefix):], nil
		}
	}
	return "", fmt.Errorf("read lsb release: could not find %s in %s", varname, filename)
}
