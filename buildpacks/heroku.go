package buildpacks

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/johnewart/archiver"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
)

//https://archive.apache.org/dist/heroku/heroku-3/3.3.3/binaries/apache-heroku-3.3.3-bin.tar.gz
const herokuDistMirror = "https://cli-assets.heroku.com/heroku-{{.OS}}-{{.Arch}}.tar.gz"

type herokuBuildTool struct {
	version string
	spec    buildToolSpec
}

func newHerokuBuildTool(toolSpec buildToolSpec) herokuBuildTool {
	tool := herokuBuildTool{
		version: toolSpec.version,
		spec:    toolSpec,
	}

	return tool
}

func (bt herokuBuildTool) archiveFile() string {
	return fmt.Sprintf("apache-heroku-%s-bin.tar.gz", bt.version)
}

func (bt herokuBuildTool) downloadURL() string {
	opsys := OS()
	arch := Arch()

	if arch == "amd64" {
		arch = "x64"
	}

	data := struct {
		OS   string
		Arch string
	}{
		opsys,
		arch,
	}

	url, _ := plumbing.TemplateToString(herokuDistMirror, data)

	return url
}

func (bt herokuBuildTool) majorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt herokuBuildTool) herokuDir() string {
	return fmt.Sprintf("%s/heroku-%s", bt.spec.packageCacheDir, bt.version)
}

func (bt herokuBuildTool) setup(ctx context.Context) error {
	herokuDir := bt.herokuDir()
	cmdPath := fmt.Sprintf("%s/heroku/bin", herokuDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	log.Infof("Setting PATH to %s", newPath)
	os.Setenv("PATH", newPath)

	return nil
}

// TODO, generalize downloader
func (bt herokuBuildTool) install(ctx context.Context) error {
	herokuDir := bt.herokuDir()

	if _, err := os.Stat(herokuDir); err == nil {
		log.Infof("Heroku v%s located in %s!", bt.version, herokuDir)
	} else {
		log.Infof("Will install Heroku v%s into %s", bt.version, herokuDir)
		downloadUrl := bt.downloadURL()

		log.Infof("Downloading Heroku from URL %s...", downloadUrl)
		localFile, err := plumbing.DownloadFileWithCache(downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err
		}
		err = archiver.Unarchive(localFile, herokuDir)
		if err != nil {
			log.Errorf("Unable to decompress: %v", err)
			return err
		}

	}

	return nil
}
