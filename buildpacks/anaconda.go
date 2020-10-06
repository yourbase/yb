package buildpacks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/yourbase/yb/plumbing"
	"zombiezen.com/go/log"
)

type anacondaBuildTool struct {
	version         string
	spec            buildToolSpec
	pyCompatibleNum int
}

const anacondaDistMirrorTemplate = "https://repo.continuum.io/miniconda/Miniconda{{.PyMajor}}-{{.Version}}-{{.OS}}-{{.Arch}}.sh"
const anacondaNewerDistMirrorTemplate = "https://repo.continuum.io/miniconda/Miniconda{{.PyMajor}}-py{{.PyMajor}}{{.PyMinor}}_{{.Version}}-{{.OS}}-{{.Arch}}.sh"

func newAnaconda2BuildTool(toolSpec buildToolSpec) anacondaBuildTool {
	tool := anacondaBuildTool{
		version:         toolSpec.version,
		spec:            toolSpec,
		pyCompatibleNum: 2,
	}

	return tool
}

func newAnaconda3BuildTool(toolSpec buildToolSpec) anacondaBuildTool {
	tool := anacondaBuildTool{
		version:         toolSpec.version,
		spec:            toolSpec,
		pyCompatibleNum: 3,
	}

	return tool
}

func (bt anacondaBuildTool) installDir() string {
	return filepath.Join(bt.spec.packageCacheDir, "miniconda", fmt.Sprintf("miniconda-%s", bt.version))
}

func (bt anacondaBuildTool) install(ctx context.Context) error {
	anacondaDir := bt.installDir()
	setupDir := bt.spec.packageDir

	if _, err := os.Stat(anacondaDir); err == nil {
		log.Infof(ctx, "anaconda installed in %s", anacondaDir)
	} else {
		log.Infof(ctx, "Installing anaconda")

		// Just so happens that for right now, Python 2.7 and 3.7
		// are the ones used for Miniconda.
		const pyMinor = 7
		downloadURL, err := anacondaDownloadURL(bt.version, bt.pyCompatibleNum, pyMinor)
		if err != nil {
			return fmt.Errorf("anaconda buildpack: %w", err)
		}

		log.Infof(ctx, "Downloading Miniconda from URL %s...", downloadURL)
		localFile, err := plumbing.DownloadFileWithCache(downloadURL)
		if err != nil {
			log.Errorf(ctx, "Unable to download: %v\n", err)
			return err
		}
		for _, cmd := range []string{
			fmt.Sprintf("chmod +x %s", localFile),
			fmt.Sprintf("bash %s -b -p %s", localFile, anacondaDir),
		} {
			log.Infof(ctx, "Running: '%v' ", cmd)
			plumbing.ExecToStdout(cmd, setupDir)
		}

	}

	return nil
}

// TODO(light): Only being shared for testing. Once OS/Arch is able to be set in
// test, we can have fixed constant strings as expected outputs.
var (
	anacondaOSMap = map[string]string{
		"darwin": "MacOSX",
		"linux":  "Linux",
		// TODO(light): Omitting Windows, since the extension also needs to change to .exe.
	}
	anacondaArchMap = map[string]string{
		"amd64": "x86_64",
		"386":   "x86",
	}
)

func anacondaDownloadURL(version string, pyMajor, pyMinor int) (string, error) {
	v, err := semver.Parse(version)
	if err != nil {
		return "", fmt.Errorf("compute anaconda %s download url: %w", version, err)
	}
	opsys := anacondaOSMap[OS()]
	if opsys == "" {
		return "", fmt.Errorf("compute anaconda %s download url: not found for OS %s", version, OS())
	}
	arch := anacondaArchMap[Arch()]
	if arch == "" {
		return "", fmt.Errorf("compute anaconda %s download url: not found for architecture %s", version, Arch())
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
		url, err := plumbing.TemplateToString(anacondaNewerDistMirrorTemplate, data)
		if err != nil {
			return "", fmt.Errorf("compute anaconda %s download url: %w", version, err)
		}
		return url, nil
	}
	url, err := plumbing.TemplateToString(anacondaDistMirrorTemplate, data)
	if err != nil {
		return "", fmt.Errorf("compute anaconda %s download url: %w", version, err)
	}
	return url, nil
}

func (bt anacondaBuildTool) setup(ctx context.Context) error {
	installDir := bt.installDir()
	plumbing.PrependToPath(filepath.Join(installDir, "bin"))
	setupDir := bt.spec.packageDir

	for _, cmd := range []string{
		"conda config --set always_yes yes",
		"conda config --set changeps1 no",
		"conda update -q conda",
	} {
		log.Infof(ctx, "Running: '%v' ", cmd)
		plumbing.ExecToStdout(cmd, setupDir)
	}

	return nil

}
