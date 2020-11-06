package buildpack

import (
	"context"
	"fmt"

	"github.com/blang/semver"
	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/ybdata"
	"zombiezen.com/go/log"
)

const anacondaDistMirrorTemplate = "https://repo.continuum.io/miniconda/Miniconda{{.PyMajor}}-{{.Version}}-{{.OS}}-{{.Arch}}.sh"
const anacondaNewerDistMirrorTemplate = "https://repo.continuum.io/miniconda/Miniconda{{.PyMajor}}-py{{.PyMajor}}{{.PyMinor}}_{{.Version}}-{{.OS}}-{{.Arch}}.sh"

func installAnaconda2(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (biome.Environment, error) {
	return installAnaconda(ctx, sys, 2, spec.Version())
}

func installAnaconda3(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (biome.Environment, error) {
	return installAnaconda(ctx, sys, 3, spec.Version())
}

func installAnaconda(ctx context.Context, sys Sys, pyMajor int, version string) (biome.Environment, error) {
	anacondaRoot := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "miniconda")
	anacondaDir := sys.Biome.JoinPath(anacondaRoot, fmt.Sprintf("miniconda-py%d-%s", pyMajor, version))

	if _, err := biome.EvalSymlinks(ctx, sys.Biome, anacondaDir); err == nil {
		log.Infof(ctx, "anaconda installed in %s", anacondaDir)
	} else {
		log.Infof(ctx, "Installing anaconda in %s", anacondaDir)

		// Just so happens that for right now, Python 2.7 and 3.7
		// are the ones used for Miniconda.
		const pyMinor = 7
		downloadURL, err := anacondaDownloadURL(version, pyMajor, pyMinor, sys.Biome.Describe())
		if err != nil {
			return biome.Environment{}, err
		}

		localScript, err := ybdata.Download(ctx, sys.HTTPClient, sys.DataDirs, downloadURL)
		if err != nil {
			return biome.Environment{}, err
		}
		defer localScript.Close()
		err = biome.MkdirAll(ctx, sys.Biome, anacondaRoot)
		if err != nil {
			return biome.Environment{}, err
		}
		scriptPath := anacondaDir + ".sh"
		err = biome.WriteFile(ctx, sys.Biome, scriptPath, localScript)
		if err != nil {
			return biome.Environment{}, err
		}
		err = sys.Biome.Run(ctx, &biome.Invocation{
			// -b: batch mode
			// -p: installation prefix
			Argv:   []string{"bash", scriptPath, "-b", "-p", anacondaDir},
			Stdout: sys.Stdout,
			Stderr: sys.Stderr,
		})
		if err != nil {
			return biome.Environment{}, fmt.Errorf("miniconda installer: %w", err)
		}
	}

	env := biome.Environment{
		PrependPath: []string{sys.Biome.JoinPath(anacondaDir, "bin")},
	}
	err := sys.Biome.Run(ctx, &biome.Invocation{
		Argv:   []string{"conda", "config", "--set", "always_yes", "yes"},
		Env:    env,
		Stdout: sys.Stdout,
		Stderr: sys.Stderr,
	})
	if err != nil {
		return biome.Environment{}, fmt.Errorf("configure miniconda: %w", err)
	}
	err = sys.Biome.Run(ctx, &biome.Invocation{
		Argv:   []string{"conda", "config", "--set", "changeps1", "no"},
		Env:    env,
		Stdout: sys.Stdout,
		Stderr: sys.Stderr,
	})
	if err != nil {
		return biome.Environment{}, fmt.Errorf("configure miniconda: %w", err)
	}
	err = sys.Biome.Run(ctx, &biome.Invocation{
		Argv:   []string{"conda", "update", "--quiet", "conda"},
		Env:    env,
		Stdout: sys.Stdout,
		Stderr: sys.Stderr,
	})
	if err != nil {
		return biome.Environment{}, fmt.Errorf("configure miniconda: %w", err)
	}
	return env, nil
}

func anacondaDownloadURL(version string, pyMajor, pyMinor int, desc *biome.Descriptor) (string, error) {
	var (
		anacondaOSMap = map[string]string{
			biome.MacOS: "MacOSX",
			biome.Linux: "Linux",
			// TODO(light): Omitting Windows, since the extension also needs to change to .exe.
		}
		anacondaArchMap = map[string]string{
			biome.Intel64: "x86_64",
			biome.Intel32: "x86",
		}
	)
	v, err := semver.Parse(version)
	if err != nil {
		return "", fmt.Errorf("compute anaconda %s download url: %w", version, err)
	}
	opsys := anacondaOSMap[desc.OS]
	if opsys == "" {
		return "", fmt.Errorf("compute anaconda %s download url: not found for OS %s", version, desc.OS)
	}
	arch := anacondaArchMap[desc.Arch]
	if arch == "" {
		return "", fmt.Errorf("compute anaconda %s download url: not found for architecture %s", version, desc.Arch)
	}

	data := struct {
		PyMajor int
		PyMinor int
		OS      string
		Arch    string
		Version string
	}{
		pyMajor,
		pyMinor,
		opsys,
		arch,
		version,
	}

	if v.Major > 4 || (v.Major == 4 && v.Minor >= 8) {
		url, err := templateToString(anacondaNewerDistMirrorTemplate, data)
		if err != nil {
			return "", fmt.Errorf("compute anaconda %s download url: %w", version, err)
		}
		return url, nil
	}
	url, err := templateToString(anacondaDistMirrorTemplate, data)
	if err != nil {
		return "", fmt.Errorf("compute anaconda %s download url: %w", version, err)
	}
	return url, nil
}
