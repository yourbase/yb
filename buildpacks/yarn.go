package buildpacks

import (
	"context"
	"path/filepath"

	"github.com/yourbase/yb/plumbing/log"
)

type YarnBuildTool struct {
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

func (bt YarnBuildTool) DownloadURL(ctx context.Context) (string, error) {
	urlTemplate := "https://github.com/yarnpkg/yarn/releases/download/v{{ .Version }}/yarn-v{{ .Version }}.tar.gz"
	data := struct {
		Version string
	}{
		bt.Version(),
	}

	url, err := TemplateToString(urlTemplate, data)
	return url, err
}

func (bt YarnBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "yarn")
	yarnDir := filepath.Join(installDir, "yarn-v"+bt.Version())

	if t.PathExists(ctx, yarnDir) {
		log.Infof("Yarn v%s located in %s!", bt.Version(), yarnDir)
		return yarnDir, nil
	}
	log.Infof("Will install Yarn v%s into %s", bt.Version(), installDir)
	downloadURL, err := bt.DownloadURL(ctx)
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}

	log.Infof("Downloading from URL %s...", downloadURL)
	localFile, err := t.DownloadFile(ctx, downloadURL)
	if err != nil {
		log.Errorf("Unable to download: %v", err)
		return "", err
	}

	if err := t.Unarchive(ctx, localFile, installDir); err != nil {
		log.Errorf("Unable to decompress: %v", err)
		return "", err
	}

	return yarnDir, nil
}

func (bt YarnBuildTool) Setup(ctx context.Context, yarnDir string) error {
	t := bt.spec.InstallTarget
	cmdPath := filepath.Join(yarnDir, "bin")
	t.PrependToPath(ctx, cmdPath)

	return nil
}
