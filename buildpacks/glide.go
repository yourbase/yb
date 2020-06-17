package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/types"
)

var GLIDE_DIST_MIRROR = "https://github.com/Masterminds/glide/releases/download/v{{.Version}}/glide-v{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz"

type GlideBuildTool struct {
	BuildTool
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

func (bt GlideBuildTool) Install(ctx context.Context) (error, string) {
	t := bt.spec.InstallTarget

	glideDir := filepath.Join(t.ToolsDir(ctx), "glide-"+bt.Version())

	if t.PathExists(ctx, glideDir) {
		log.Infof("Glide v%s located in %s!", bt.Version(), glideDir)
	} else {
		log.Infof("Will install Glide v%s into %s", bt.Version(), glideDir)
		params := DownloadParameters{
			OS:      OS(),
			Arch:    Arch(),
			Version: bt.Version(),
		}

		downloadUrl, err := TemplateToString(GLIDE_DIST_MIRROR, params)
		if err != nil {
			log.Errorf("Unable to generate download URL: %v", err)
			return err, ""
		}

		log.Infof("Downloading from URL %s ...", downloadUrl)
		localFile, err := t.DownloadFile(ctx, downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err, ""
		}

		t.MkdirAsNeeded(ctx, glideDir)
		log.Infof("Extracting glide %s to %s...", bt.Version(), glideDir)
		err = t.Unarchive(ctx, localFile, glideDir)
		if err != nil {
			log.Errorf("Unable to decompress: %v", err)
			return err, ""
		}

	}

	return nil, glideDir

}
