package buildpacks

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnewart/archiver"
	"github.com/yourbase/yb/plumbing"
	"zombiezen.com/go/log"
)

//https://archive.apache.org/dist/dart/dart-3/3.3.3/binaries/apache-dart-3.3.3-bin.tar.gz
const dartDistMirror = "https://storage.googleapis.com/dart-archive/channels/stable/release/{{.Version}}/sdk/dartsdk-{{.OS}}-{{.Arch}}-release.zip"

type dartBuildTool struct {
	version string
	spec    buildToolSpec
}

func newDartBuildTool(toolSpec buildToolSpec) dartBuildTool {
	tool := dartBuildTool{
		version: toolSpec.version,
		spec:    toolSpec,
	}

	return tool
}

func (bt dartBuildTool) downloadURL() string {
	opsys := OS()
	arch := Arch()
	extension := "zip"

	if arch == "amd64" {
		arch = "x64"
	}

	if opsys == "darwin" {
		opsys = "macos"
	}

	version := bt.version

	data := struct {
		OS        string
		Arch      string
		Version   string
		Extension string
	}{
		opsys,
		arch,
		version,
		extension,
	}

	url, _ := plumbing.TemplateToString(dartDistMirror, data)

	return url
}

func (bt dartBuildTool) majorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt dartBuildTool) installDir() string {
	return filepath.Join(bt.spec.cacheDir, "dart", bt.version)
}

func (bt dartBuildTool) dartDir() string {
	return filepath.Join(bt.installDir(), "dart-sdk")
}

func (bt dartBuildTool) setup(ctx context.Context) error {
	dartDir := bt.dartDir()
	cmdPath := filepath.Join(dartDir, "bin")
	plumbing.PrependToPath(cmdPath)

	return nil
}

// TODO, generalize downloader
func (bt dartBuildTool) install(ctx context.Context) error {
	dartDir := bt.dartDir()
	installDir := bt.installDir()

	if _, err := os.Stat(dartDir); err == nil {
		log.Infof(ctx, "Dart v%s located in %s!", bt.version, dartDir)
	} else {
		log.Infof(ctx, "Will install Dart v%s into %s", bt.version, dartDir)
		downloadURL := bt.downloadURL()

		log.Infof(ctx, "Downloading Dart from URL %s...", downloadURL)
		localFile, err := plumbing.DownloadFileWithCache(ctx, http.DefaultClient, bt.spec.dataDirs, downloadURL)
		if err != nil {
			log.Errorf(ctx, "Unable to download: %v", err)
			return err
		}
		err = archiver.Unarchive(localFile, installDir)
		if err != nil {
			log.Errorf(ctx, "Unable to decompress: %v", err)
			return err
		}
	}

	return nil
}
