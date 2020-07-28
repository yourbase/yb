package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/types"
)

type YarnBuildTool struct {
	BuildTool
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

func (bt YarnBuildTool) DownloadUrl() string {
	urlTemplate := "https://github.com/yarnpkg/yarn/releases/download/v{{ .Version }}/yarn-v{{ .Version }}.tar.gz"
	data := struct {
		Version string
	}{
		bt.Version(),
	}

	url, _ := TemplateToString(urlTemplate, data)

	return url
}

func (bt YarnBuildTool) Install(ctx context.Context) (error, string) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "yarn")
	yarnDir := filepath.Join(installDir, "yarn-v"+bt.Version())

	if t.PathExists(ctx, yarnDir) {
		log.Infof("Yarn v%s located in %s!", bt.Version(), yarnDir)
	} else {
		log.Infof("Will install Yarn v%s into %s", bt.Version(), installDir)
		downloadUrl := bt.DownloadUrl()
		log.Infof("Downloading from URL %s...", downloadUrl)
		localFile, err := t.DownloadFile(ctx, downloadUrl)
		if err != nil {
			return fmt.Errorf("Unable to download %s: %v", downloadUrl, err), ""
		}

		if err := t.Unarchive(ctx, localFile, installDir); err != nil {
			return fmt.Errorf("Unable to decompress archive: %v", err), ""
		}
	}

	return nil, yarnDir
}

func (bt YarnBuildTool) Setup(ctx context.Context, yarnDir string) error {
	t := bt.spec.InstallTarget
	cmdPath := filepath.Join(yarnDir, "bin")
	t.PrependToPath(ctx, cmdPath)

	return nil
}
