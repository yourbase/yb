package buildpacks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/johnewart/archiver"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
)

type yarnBuildTool struct {
	version string
	spec    buildToolSpec
}

func newYarnBuildTool(toolSpec buildToolSpec) yarnBuildTool {
	tool := yarnBuildTool{
		version: toolSpec.version,
		spec:    toolSpec,
	}

	return tool
}

func (bt yarnBuildTool) yarnDir() string {
	return filepath.Join(bt.installDir(), fmt.Sprintf("yarn-v%s", bt.version))
}

func (bt yarnBuildTool) installDir() string {
	return filepath.Join(bt.spec.sharedCacheDir, "yarn")
}

func (bt yarnBuildTool) downloadURL() string {
	urlTemplate := "https://github.com/yarnpkg/yarn/releases/download/v{{ .Version }}/yarn-v{{ .Version }}.tar.gz"
	data := struct {
		Version string
	}{
		bt.version,
	}

	url, _ := plumbing.TemplateToString(urlTemplate, data)

	return url
}

func (bt yarnBuildTool) install(ctx context.Context) error {

	yarnDir := bt.yarnDir()
	installDir := bt.installDir()

	if _, err := os.Stat(yarnDir); err == nil {
		log.Infof("Yarn v%s located in %s!", bt.version, yarnDir)
	} else {
		log.Infof("Will install Yarn v%s into %s", bt.version, installDir)
		downloadUrl := bt.downloadURL()
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

func (bt yarnBuildTool) setup(ctx context.Context) error {
	yarnDir := bt.yarnDir()
	cmdPath := filepath.Join(yarnDir, "bin")
	plumbing.PrependToPath(cmdPath)

	return nil
}
