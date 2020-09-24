package buildpacks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
)

type anacondaBuildTool struct {
	version         string
	spec            buildToolSpec
	pyCompatibleNum int
}

const anacondaDistMirrorTemplate = "https://repo.continuum.io/miniconda/Miniconda{{.PyNum}}-{{.Version}}-{{.OS}}-{{.Arch}}.{{.Extension}}"
const anacondaNewerDistMirrorTemplate = "https://repo.continuum.io/miniconda/Miniconda{{.PyNum}}-{{.PyMajorVersion}}_{{.Version}}-{{.OS}}-{{.Arch}}.{{.Extension}}"

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
		log.Infof("anaconda installed in %s", anacondaDir)
	} else {
		log.Infof("Installing anaconda")

		downloadUrl := bt.downloadURL()

		log.Infof("Downloading Miniconda from URL %s...", downloadUrl)
		localFile, err := plumbing.DownloadFileWithCache(downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v\n", err)
			return err
		}
		for _, cmd := range []string{
			fmt.Sprintf("chmod +x %s", localFile),
			fmt.Sprintf("bash %s -b -p %s", localFile, anacondaDir),
		} {
			log.Infof("Running: '%v' ", cmd)
			plumbing.ExecToStdout(cmd, setupDir)
		}

	}

	return nil
}

func (bt anacondaBuildTool) downloadURL() string {
	var v semver.Version

	opsys := OS()
	arch := Arch()
	extension := "sh"
	version := bt.version

	if version == "" {
		version = "latest"
	} else {
		var errv error
		v, errv = semver.Parse(version)
		if errv != nil {
			return ""
		}
	}

	if arch == "amd64" {
		arch = "x86_64"
	}

	if opsys == "darwin" {
		opsys = "MacOSX"
	}

	if opsys == "linux" {
		opsys = "Linux"
	}

	if opsys == "windows" {
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
		url, err := plumbing.TemplateToString(anacondaNewerDistMirrorTemplate, data)
		log.Errorf("Unable to apply template: %v", err)
		return url
	}
	url, err := plumbing.TemplateToString(anacondaDistMirrorTemplate, data)
	log.Errorf("Unable to apply template: %v", err)

	return url
}

func (bt anacondaBuildTool) setup(ctx context.Context) error {
	installDir := bt.installDir()
	plumbing.PrependToPath(filepath.Join(installDir, "bin"))
	setupDir := bt.spec.packageDir

	for _, cmd := range []string{
		"conda config --set always_yes yes --set changeps1 no",
		"conda update -q conda",
	} {
		log.Infof("Running: '%v' ", cmd)
		plumbing.ExecToStdout(cmd, setupDir)
	}

	return nil

}
