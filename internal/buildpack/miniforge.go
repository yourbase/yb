package buildpack

import (
	"context"
	"fmt"

	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log"
)

// Miniforge3-Darwin-x86_64.sh
//https://github.com/conda-forge/miniforge/releases/download/4.10.1-4/Miniforge3-Darwin-x86_64.sh

const miniforgeDistMirrorTemplate = "https://github.com/conda-forge/miniforge/releases/download/4.10.1-4/Miniforge3-{{ .OS }}-{{ .Arch }}.sh"

func installMiniforge(ctx context.Context, sys Sys) (biome.Environment, error) {
	miniforgeRoot := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "miniforge")
	miniforgeDir := sys.Biome.JoinPath(miniforgeRoot, "miniforge")

	if _, err := biome.EvalSymlinks(ctx, sys.Biome, miniforgeDir); err == nil {
		log.Infof(ctx, "miniforge installed in %s", miniforgeDir)
	} else {
		log.Infof(ctx, "Installing miniforge in %s", miniforgeDir)

		downloadURL, err := miniforgeDownloadURL(sys.Biome.Describe())
		if err != nil {
			return biome.Environment{}, err
		}

		localScript, err := sys.Downloader.Download(ctx, downloadURL)
		if err != nil {
			return biome.Environment{}, err
		}
		defer localScript.Close()
		err = biome.MkdirAll(ctx, sys.Biome, miniforgeRoot)
		if err != nil {
			return biome.Environment{}, err
		}
		scriptPath := miniforgeDir + ".sh"
		err = biome.WriteFile(ctx, sys.Biome, scriptPath, localScript)
		if err != nil {
			return biome.Environment{}, err
		}
		err = sys.Biome.Run(ctx, &biome.Invocation{
			// -b: batch mode
			// -p: installation prefix
			Argv:   []string{"bash", scriptPath, "-b", "-p", miniforgeDir},
			Stdout: sys.Stdout,
			Stderr: sys.Stderr,
		})
		if err != nil {
			return biome.Environment{}, fmt.Errorf("miniforge installer: %w", err)
		}
	}

	env := biome.Environment{
		PrependPath: []string{sys.Biome.JoinPath(miniforgeDir, "bin")},
	}
	err := sys.Biome.Run(ctx, &biome.Invocation{
		Argv:   []string{"conda", "config", "--set", "always_yes", "yes"},
		Env:    env,
		Stdout: sys.Stdout,
		Stderr: sys.Stderr,
	})
	if err != nil {
		return biome.Environment{}, fmt.Errorf("configure miniforge: %w", err)
	}
	err = sys.Biome.Run(ctx, &biome.Invocation{
		Argv:   []string{"conda", "config", "--set", "changeps1", "no"},
		Env:    env,
		Stdout: sys.Stdout,
		Stderr: sys.Stderr,
	})
	if err != nil {
		return biome.Environment{}, fmt.Errorf("configure miniforge: %w", err)
	}
	err = sys.Biome.Run(ctx, &biome.Invocation{
		Argv:   []string{"conda", "update", "--quiet", "conda"},
		Env:    env,
		Stdout: sys.Stdout,
		Stderr: sys.Stderr,
	})
	if err != nil {
		return biome.Environment{}, fmt.Errorf("configure miniforge: %w", err)
	}
	return env, nil
}

func miniforgeDownloadURL(desc *biome.Descriptor) (string, error) {
	var (
		miniforgeOSMap = map[string]string{
			biome.MacOS: "MacOSX",
			biome.Linux: "Linux",
			// TODO(light): Omitting Windows, since the extension also needs to change to .exe.
		}
		miniforgeArchMap = map[string]string{
			biome.Intel64: "x86_64",
			biome.Intel32: "x86",
			biome.Arm64:   "arm64",
		}
	)
	opsys := miniforgeOSMap[desc.OS]
	if opsys == "" {
		return "", fmt.Errorf("compute miniforge download url: not found for OS %s", desc.OS)
	}
	arch := miniforgeArchMap[desc.Arch]
	if arch == "" {
		return "", fmt.Errorf("compute miniforge download url: not found for architecture %s", desc.Arch)
	}

	data := struct {
		OS   string
		Arch string
	}{
		opsys,
		arch,
	}

	url, err := templateToString(miniforgeDistMirrorTemplate, data)
	if err != nil {
		return "", fmt.Errorf("compute miniforge download url: %w", err)
	}
	return url, nil
}
