package buildpacks

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/johnewart/archiver"
	. "github.com/yourbase/yb/plumbing"
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
	cmdPath := fmt.Sprintf("%s/%s-%s", glideDir, OS(), Arch())
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	fmt.Printf("Setting PATH to %s\n", newPath)
	os.Setenv("PATH", newPath)

	return nil
}

func (bt GlideBuildTool) Install() error {

	glidePath := bt.GlideDir()

	if _, err := os.Stat(glidePath); err == nil {
		fmt.Printf("Glide v%s located in %s!\n", bt.Version(), glidePath)
	} else {
		fmt.Printf("Will install Glide v%s into %s\n", bt.Version(), glidePath)
		params := DownloadParameters{
			OS:      OS(),
			Arch:    Arch(),
			Version: bt.Version(),
		}

		downloadUrl, err := TemplateToString(GLIDE_DIST_MIRROR, params)
		if err != nil {
			fmt.Printf("Unable to generate download URL: %v\n", err)
			return err
		}

		fmt.Printf("Downloading from URL %s ...\n", downloadUrl)
		localFile, err := DownloadFileWithCache(downloadUrl)
		if err != nil {
			fmt.Printf("Unable to download: %v\n", err)
			return err
		}

		extractDir := bt.GlideDir()
		MkdirAsNeeded(extractDir)
		fmt.Printf("Extracting glide %s to %s...\n", bt.Version(), extractDir)
		err = archiver.Unarchive(localFile, extractDir)
		if err != nil {
			fmt.Printf("Unable to decompress: %v\n", err)
			return err
		}

	}

	return nil

}
