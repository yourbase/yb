package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/yourbase/yb/plumbing/log"
)

const glideDistMirrorTemplate = "https://github.com/Masterminds/glide/releases/download/v{{.Version}}/glide-v{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz"

type GlideBuildTool struct {
	version string
	spec    BuildToolSpec
}

type DownloadParameters struct {
	Version string
	OS      string
	Arch    string
}

func NewGlideBuildTool(toolSpec BuildToolSpec) GlideBuildTool {
	tool := GlideBuildTool{
		version: toolSpec.Version,
		spec:    toolSpec,
	}

	return tool
}

func (bt GlideBuildTool) Version() string {
	return bt.version
}

func (bt GlideBuildTool) Setup(ctx context.Context, glideDir string) error {
	t := bt.spec.InstallTarget

	cmdPath := filepath.Join(glideDir, fmt.Sprintf("%s-%s", OS(), Arch()))

	t.PrependToPath(ctx, cmdPath)

	return nil
}

func (bt GlideBuildTool) DownloadURL(ctx context.Context) (string, error) {
	params := struct {
		OS      string
		Arch    string
		Version string
	}{
		OS:      OS(),
		Arch:    Arch(),
		Version: bt.Version(),
	}

	downloadURL, err := TemplateToString(glideDistMirrorTemplate, params)
	return downloadURL, err
}

func (bt GlideBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	glideDir := filepath.Join(t.ToolsDir(ctx), "glide-"+bt.Version())

	if t.PathExists(ctx, glideDir) {
		log.Infof("Glide v%s located in %s!", bt.Version(), glideDir)
		return glideDir, nil
	}
	log.Infof("Will install Glide v%s into %s", bt.Version(), glideDir)
	downloadURL, err := bt.DownloadURL(ctx)
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}

	log.Infof("Downloading from URL %s ...", downloadURL)
	localFile, err := t.DownloadFile(ctx, downloadURL)
	if err != nil {
		log.Errorf("Unable to download: %v", err)
		return "", err
	}

	t.MkdirAsNeeded(ctx, glideDir)
	log.Infof("Extracting glide %s to %s...", bt.Version(), glideDir)
	err = t.Unarchive(ctx, localFile, glideDir)
	if err != nil {
		log.Errorf("Unable to decompress: %v", err)
		return "", err
	}

	return glideDir, nil

}
