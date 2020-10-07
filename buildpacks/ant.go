package buildpacks

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnewart/archiver"
	"github.com/yourbase/yb/plumbing"
	"zombiezen.com/go/log"
)

//http://apache.mirrors.lucidnetworks.net//ant/binaries/apache-ant-1.10.6-bin.tar.gz
const antDistMirror = "http://apache.mirrors.lucidnetworks.net/ant/binaries/apache-ant-{{.Version}}-bin.zip"

type antBuildTool struct {
	version string
	spec    buildToolSpec
}

func newAntBuildTool(toolSpec buildToolSpec) antBuildTool {

	tool := antBuildTool{
		version: toolSpec.version,
		spec:    toolSpec,
	}

	return tool
}

func (bt antBuildTool) archiveFile() string {
	return fmt.Sprintf("apache-ant-%s-bin.zip", bt.version)
}

func (bt antBuildTool) downloadURL() string {
	data := struct {
		Version string
	}{
		bt.version,
	}

	url, _ := plumbing.TemplateToString(antDistMirror, data)

	return url
}

func (bt antBuildTool) majorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt antBuildTool) antDir() string {
	return filepath.Join(bt.installDir(), fmt.Sprintf("apache-ant-%s", bt.version))
}

func (bt antBuildTool) installDir() string {
	return filepath.Join(bt.spec.sharedCacheDir, "ant")
}

func (bt antBuildTool) setup(ctx context.Context) error {
	antDir := bt.antDir()

	cmdPath := filepath.Join(antDir, "bin")
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	log.Infof(ctx, "Setting PATH to %s", newPath)
	os.Setenv("PATH", newPath)

	return nil
}

// TODO, generalize downloader
func (bt antBuildTool) install(ctx context.Context) error {
	antDir := bt.antDir()
	installDir := bt.installDir()

	if _, err := os.Stat(antDir); err == nil {
		log.Infof(ctx, "Ant v%s located in %s!", bt.version, antDir)
	} else {
		log.Infof(ctx, "Will install Ant v%s into %s", bt.version, antDir)
		downloadURL := bt.downloadURL()

		log.Infof(ctx, "Downloading Ant from URL %s...", downloadURL)
		localFile, err := plumbing.DownloadFileWithCache(ctx, http.DefaultClient, downloadURL)
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
