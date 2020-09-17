package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/blang/semver"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
)

type AnacondaBuildTool struct {
	version         string
	spec            BuildToolSpec
	pyCompatibleNum int
}

const anacondaDistMirrorTemplate = "https://repo.continuum.io/miniconda/Miniconda{{.PyNum}}-{{.Version}}-{{.OS}}-{{.Arch}}.{{.Extension}}"
const anacondaNewerDistMirrorTemplate = "https://repo.continuum.io/miniconda/Miniconda{{.PyNum}}-{{.PyMajorVersion}}_{{.Version}}-{{.OS}}-{{.Arch}}.{{.Extension}}"

func NewAnaconda2BuildTool(toolSpec BuildToolSpec) AnacondaBuildTool {
	tool := AnacondaBuildTool{
		version:         toolSpec.Version,
		spec:            toolSpec,
		pyCompatibleNum: 2,
	}

	return tool
}

func NewAnaconda3BuildTool(toolSpec BuildToolSpec) AnacondaBuildTool {
	tool := AnacondaBuildTool{
		version:         toolSpec.Version,
		spec:            toolSpec,
		pyCompatibleNum: 3,
	}

	return tool
}

func (bt AnacondaBuildTool) Version() string {
	return bt.version
}

func (bt AnacondaBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	anacondaDir := filepath.Join(t.ToolsDir(ctx), "miniconda", "miniconda-"+bt.Version())
	setupDir := bt.spec.PackageDir

	if t.PathExists(ctx, anacondaDir) {
		log.Infof("Anaconda installed in %s", anacondaDir)
		return anacondaDir, nil
	}
	log.Infof("Installing anaconda")

	downloadURL, err := bt.DownloadURL(ctx)
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}

	log.Infof("Downloading Miniconda from URL %s...", downloadURL)
	localFile, err := t.DownloadFile(ctx, downloadURL)
	if err != nil {
		log.Errorf("Unable to download: %v\n", err)
		return "", err
	}
	for _, cmd := range []string{
		fmt.Sprintf("chmod +x %s", localFile),
		fmt.Sprintf("bash %s -b -p %s", localFile, anacondaDir),
	} {
		log.Infof("Running: '%v' ", cmd)
		t.Run(ctx, runtime.Process{
			Command:   cmd,
			Directory: setupDir,
		})
	}

	return anacondaDir, nil
}

func (bt AnacondaBuildTool) DownloadURL(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget
	var v semver.Version

	opsys := "linux"
	arch := "x86_64"
	extension := "sh"
	os := t.OS()
	architecture := t.Architecture()
	version := bt.Version()

	if version == "" {
		version = "latest"
	} else {
		var errv error
		v, errv = semver.Parse(version)
		if errv != nil {
			return "", fmt.Errorf("parsing semver of '%s': %v", version, errv)
		}
	}

	if architecture == runtime.Amd64 {
		arch = "x86_64"
	}

	if os == runtime.Darwin {
		opsys = "MacOSX"
	}

	if os == runtime.Linux {
		opsys = "Linux"
	}

	if os == runtime.Windows {
		opsys = "Windows"
		extension = "exe"
	}

	data := struct {
		PyNum          int
		PyMajorVersion string
		OS             string
		Arch           string
		Version        string
		Extension      string
	}{
		bt.pyCompatibleNum,
		"py37",
		opsys,
		arch,
		version,
		extension,
	}

	// Newest Miniconda installs has different installers for Python 3.7 and Python 3.8
	// We'll stick to Python 3.7, the stable version right now
	if v.Major == 4 && v.Minor == 8 {
		url, err := TemplateToString(anacondaNewerDistMirrorTemplate, data)
		return url, err
	}
	url, err := TemplateToString(anacondaDistMirrorTemplate, data)

	return url, err
}

func (bt AnacondaBuildTool) Setup(ctx context.Context, installDir string) error {
	t := bt.spec.InstallTarget

	t.PrependToPath(ctx, filepath.Join(installDir, "bin"))
	setupDir := bt.spec.PackageDir

	for _, cmd := range []string{
		fmt.Sprintf("conda config --set always_yes yes --set changeps1 no"),
		fmt.Sprintf("conda update -q conda"),
	} {
		log.Infof("Running: '%v' ", cmd)
		err := t.Run(ctx, runtime.Process{
			Command:   cmd,
			Directory: setupDir,
		})
		if err != nil {
			return err
		}
	}

	return nil

}
