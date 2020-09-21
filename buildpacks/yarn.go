package buildpacks

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/johnewart/archiver"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/types"
)

type YarnBuildTool struct {
	types.BuildTool
	version string
	spec    BuildToolSpec
}

func NewYarnBuildTool(toolSpec BuildToolSpec) YarnBuildTool {
	tool := YarnBuildTool{
		version: toolSpec.Version,
		spec:    toolSpec,
	}

	return tool
}

func (bt YarnBuildTool) Version() string {
	return bt.version
}

func (bt YarnBuildTool) YarnDir() string {
	return filepath.Join(bt.InstallDir(), fmt.Sprintf("yarn-v%s", bt.Version()))
}

func (bt YarnBuildTool) InstallDir() string {
	return filepath.Join(bt.spec.SharedCacheDir, "yarn")
}

func (bt YarnBuildTool) DownloadUrl() string {
	urlTemplate := "https://github.com/yarnpkg/yarn/releases/download/v{{ .Version }}/yarn-v{{ .Version }}.tar.gz"
	data := struct {
		Version string
	}{
		bt.Version(),
	}

	url, _ := plumbing.TemplateToString(urlTemplate, data)

	return url
}

func (bt YarnBuildTool) Install() error {

	yarnDir := bt.YarnDir()
	installDir := bt.InstallDir()

	if _, err := os.Stat(yarnDir); err == nil {
		log.Infof("Yarn v%s located in %s!", bt.Version(), yarnDir)
	} else {
		log.Infof("Will install Yarn v%s into %s", bt.Version(), installDir)
		downloadUrl := bt.DownloadUrl()
		log.Infof("Downloading from URL %s...", downloadUrl)
		localFile, err := plumbing.DownloadFileWithCache(downloadUrl)
		if err != nil {
			return fmt.Errorf("Unable to download %s: %v", downloadUrl, err)
		}

		if err := archiver.Unarchive(localFile, installDir); err != nil {
			return fmt.Errorf("Unable to decompress archive: %v", err)
		}
	}

	return nil
}

func (bt YarnBuildTool) Setup() error {
	yarnDir := bt.YarnDir()
	cmdPath := filepath.Join(yarnDir, "bin")
	plumbing.PrependToPath(cmdPath)

	return nil
}
