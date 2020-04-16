package buildpacks

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/yourbase/yb/plumbing"
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

func (bt GlideBuildTool) GlideDir() string {
	return filepath.Join(bt.spec.SharedCacheDir, fmt.Sprintf("glide-%v", bt.Version()))
}

func (bt GlideBuildTool) Setup() error {
	glideDir := bt.GlideDir()

	t := bt.spec.InstallTarget
	cmdPath := filepath.Join(glideDir, fmt.Sprintf("%s-%s", OS(), Arch()))

	t.PrependToPath(cmdPath)

	return nil
}

func (bt GlideBuildTool) Install() error {

	glidePath := bt.GlideDir()

	if _, err := os.Stat(glidePath); err == nil {
		log.Infof("Glide v%s located in %s!", bt.Version(), glidePath)
	} else {
		log.Infof("Will install Glide v%s into %s", bt.Version(), glidePath)
		params := DownloadParameters{
			OS:      OS(),
			Arch:    Arch(),
			Version: bt.Version(),
		}

		downloadUrl, err := TemplateToString(GLIDE_DIST_MIRROR, params)
		if err != nil {
			log.Errorf("Unable to generate download URL: %v", err)
			return err
		}

		log.Infof("Downloading from URL %s ...", downloadUrl)
		localFile, err := bt.spec.InstallTarget.DownloadFile(downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err
		}

		extractDir := bt.GlideDir()
		MkdirAsNeeded(extractDir)
		log.Infof("Extracting glide %s to %s...", bt.Version(), extractDir)
		err = bt.spec.InstallTarget.Unarchive(localFile, extractDir)
		if err != nil {
			log.Errorf("Unable to decompress: %v", err)
			return err
		}

	}

	return nil

}
