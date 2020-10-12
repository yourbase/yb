package buildpack

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/johnewart/archiver"
	"github.com/yourbase/yb/plumbing"
	"zombiezen.com/go/log"
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
	return filepath.Join(bt.spec.cacheDir, "yarn")
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
		log.Infof(ctx, "Yarn v%s located in %s!", bt.version, yarnDir)
	} else {
		log.Infof(ctx, "Will install Yarn v%s into %s", bt.version, installDir)
		downloadURL := bt.downloadURL()
		log.Infof(ctx, "Downloading from URL %s...", downloadURL)
		localFile, err := plumbing.DownloadFileWithCache(ctx, http.DefaultClient, bt.spec.dataDirs, downloadURL)
		if err != nil {
			return fmt.Errorf("Unable to download %s: %v", downloadURL, err)
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
