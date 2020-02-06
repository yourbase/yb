package buildpacks

import (
	"fmt"
	"github.com/yourbase/yb/runtime"
	"os"
	"strings"

	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/types"
)

//https://archive.apache.org/dist/heroku/heroku-3/3.3.3/binaries/apache-heroku-3.3.3-bin.tar.gz
var HEROKU_DIST_MIRROR = "https://cli-assets.heroku.com/heroku-{{.OS}}-{{.Arch}}.tar.gz"

type HerokuBuildTool struct {
	BuildTool
	version string
	spec    BuildToolSpec
}

func NewHerokuBuildTool(toolSpec BuildToolSpec) HerokuBuildTool {
	tool := HerokuBuildTool{
		version: toolSpec.Version,
		spec:    toolSpec,
	}

	return tool
}

func (bt HerokuBuildTool) ArchiveFile() string {
	return fmt.Sprintf("apache-heroku-%s-bin.tar.gz", bt.Version())
}

func (bt HerokuBuildTool) DownloadUrl() string {
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

	url, _ := TemplateToString(HEROKU_DIST_MIRROR, data)

	return url
}

func (bt HerokuBuildTool) MajorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt HerokuBuildTool) Version() string {
	return bt.version
}

func (bt HerokuBuildTool) HerokuDir() string {
	return fmt.Sprintf("%s/heroku-%s", bt.spec.PackageCacheDir, bt.Version())
}

func (bt HerokuBuildTool) Setup() error {
	herokuDir := bt.HerokuDir()
	cmdPath := fmt.Sprintf("%s/heroku/bin", herokuDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	log.Infof("Setting PATH to %s", newPath)
	runtime.SetEnv("PATH", newPath)

	return nil
}

// TODO, generalize downloader
func (bt HerokuBuildTool) Install() error {
	herokuDir := bt.HerokuDir()

	if bt.spec.InstallTarget.PathExists(herokuDir) {
		log.Infof("Heroku v%s located in %s!", bt.Version(), herokuDir)
	} else {
		log.Infof("Will install Heroku v%s into %s", bt.Version(), herokuDir)
		downloadUrl := bt.DownloadUrl()

		log.Infof("Downloading Heroku from URL %s...", downloadUrl)
		localFile, err := bt.spec.InstallTarget.DownloadFile(downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err
		}
		err = bt.spec.InstallTarget.Unarchive(localFile, herokuDir)
		if err != nil {
			log.Errorf("Unable to decompress: %v", err)
			return err
		}

	}

	return nil
}
