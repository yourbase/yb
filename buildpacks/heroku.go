package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"
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

func (bt HerokuBuildTool) Setup(ctx context.Context, herokuDir string) error {
	t := bt.spec.InstallTarget

	t.PrependToPath(ctx, filepath.Join(herokuDir, "heroku", "bin"))

	return nil
}

func (bt HerokuBuildTool) Install(ctx context.Context) (error, string) {
	t := bt.spec.InstallTarget

	herokuDir := filepath.Join(t.ToolsDir(ctx), "heroku", bt.Version())

	if t.PathExists(ctx, herokuDir) {
		log.Infof("Heroku v%s located in %s!", bt.Version(), herokuDir)
	} else {
		log.Infof("Will install Heroku v%s into %s", bt.Version(), herokuDir)
		downloadUrl := bt.DownloadUrl()

		log.Infof("Downloading Heroku from URL %s...", downloadUrl)
		localFile, err := t.DownloadFile(ctx, downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err, ""
		}
		err = t.Unarchive(ctx, localFile, herokuDir)
		if err != nil {
			log.Errorf("Unable to decompress: %v", err)
			return err, ""
		}

	}

	return nil, herokuDir
}
