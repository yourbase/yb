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

const glideDistMirror = "https://github.com/Masterminds/glide/releases/download/v{{.Version}}/glide-v{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz"

type glideBuildTool struct {
	version string
	spec    buildToolSpec
}

type downloadParameters struct {
	Version string
	OS      string
	Arch    string
}

func newGlideBuildTool(toolSpec buildToolSpec) glideBuildTool {
	tool := glideBuildTool{
		version: toolSpec.version,
		spec:    toolSpec,
	}

	return tool
}

func (bt glideBuildTool) glideDir() string {
	return filepath.Join(bt.spec.cacheDir, fmt.Sprintf("glide-%v", bt.version))
}

func (bt glideBuildTool) setup(ctx context.Context) error {
	glideDir := bt.glideDir()
	cmdPath := fmt.Sprintf("%s/%s-%s", glideDir, OS(), Arch())
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	log.Infof(ctx, "Setting PATH to %s", newPath)
	os.Setenv("PATH", newPath)

	return nil
}

func (bt glideBuildTool) install(ctx context.Context) error {

	glidePath := bt.glideDir()

	if _, err := os.Stat(glidePath); err == nil {
		log.Infof(ctx, "Glide v%s located in %s!", bt.version, glidePath)
	} else {
		log.Infof(ctx, "Will install Glide v%s into %s", bt.version, glidePath)
		params := downloadParameters{
			OS:      OS(),
			Arch:    Arch(),
			Version: bt.version,
		}

		downloadURL, err := plumbing.TemplateToString(glideDistMirror, params)
		if err != nil {
			log.Errorf(ctx, "Unable to generate download URL: %v", err)
			return err
		}

		log.Infof(ctx, "Downloading from URL %s ...", downloadURL)
		localFile, err := plumbing.DownloadFileWithCache(ctx, http.DefaultClient, bt.spec.dataDirs, downloadURL)
		if err != nil {
			log.Errorf(ctx, "Unable to download: %v", err)
			return err
		}

		extractDir := bt.glideDir()
		if err := os.MkdirAll(extractDir, 0777); err != nil {
			return err
		}
		log.Infof(ctx, "Extracting glide %s to %s...", bt.version, extractDir)
		err = archiver.Unarchive(localFile, extractDir)
		if err != nil {
			log.Errorf(ctx, "Unable to decompress: %v", err)
			return err
		}

	}

	return nil

}
