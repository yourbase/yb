package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yourbase/yb/plumbing/log"
)

//https://archive.apache.org/dist/heroku/heroku-3/3.3.3/binaries/apache-heroku-3.3.3-bin.tar.gz
const herokuDistMirrorTemplate = "https://cli-assets.heroku.com/heroku-{{.OS}}-{{.Arch}}.tar.gz"

type HerokuBuildTool struct {
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

func (bt HerokuBuildTool) DownloadURL(ctx context.Context) (string, error) {
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

	url, err := TemplateToString(herokuDistMirrorTemplate, data)
	return url, err
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

func (bt HerokuBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	herokuDir := filepath.Join(t.ToolsDir(ctx), "heroku", bt.Version())

	if t.PathExists(ctx, herokuDir) {
		log.Infof("Heroku v%s located in %s!", bt.Version(), herokuDir)
		return herokuDir, nil
	}
	log.Infof("Will install Heroku v%s into %s", bt.Version(), herokuDir)
	downloadURL, err := bt.DownloadURL(ctx)
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}

	log.Infof("Downloading Heroku from URL %s...", downloadURL)
	localFile, err := t.DownloadFile(ctx, downloadURL)
	if err != nil {
		log.Errorf("Unable to download: %v", err)
		return "", err
	}
	err = t.Unarchive(ctx, localFile, herokuDir)
	if err != nil {
		log.Errorf("Unable to decompress: %v", err)
		return "", err
	}

	return herokuDir, err
}
