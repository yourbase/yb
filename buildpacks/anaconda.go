package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
	. "github.com/yourbase/yb/types"
)

type AnacondaBuildTool struct {
	BuildTool
	version         string
	spec            BuildToolSpec
	pyCompatibleNum int
}

var ANACONDA_DIST_MIRROR = "https://repo.continuum.io/miniconda/Miniconda{{.PyNum}}-{{.Version}}-{{.OS}}-{{.Arch}}.{{.Extension}}"

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

func (bt AnacondaBuildTool) Install(ctx context.Context) (error, string) {
	t := bt.spec.InstallTarget

	anacondaDir := filepath.Join(t.ToolsDir(ctx), "miniconda", "miniconda-"+bt.Version())
	setupDir := bt.spec.PackageDir

	if t.PathExists(ctx, anacondaDir) {
		log.Infof("Anaconda installed in %s", anacondaDir)
	} else {
		log.Infof("Installing anaconda")

		downloadUrl := bt.DownloadUrl()

		log.Infof("Downloading Miniconda from URL %s...", downloadUrl)
		localFile, err := t.DownloadFile(ctx, downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v\n", err)
			return err, ""
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

	}

	return nil, anacondaDir
}

func (bt AnacondaBuildTool) DownloadUrl() string {
	opsys := OS()
	arch := Arch()
	extension := "sh"
	version := bt.Version()

	if version == "" {
		version = "latest"
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
		PyNum     int
		OS        string
		Arch      string
		Version   string
		Extension string
	}{
		bt.pyCompatibleNum,
		opsys,
		arch,
		version,
		extension,
	}

	url, _ := TemplateToString(ANACONDA_DIST_MIRROR, data)

	return url
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
