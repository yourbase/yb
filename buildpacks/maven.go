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

//https://archive.apache.org/dist/maven/maven-3/3.3.3/binaries/apache-maven-3.3.3-bin.tar.gz
const mavenDistMirror = "https://archive.apache.org/dist/maven/"

type mavenBuildTool struct {
	version string
	spec    buildToolSpec
}

func newMavenBuildTool(toolSpec buildToolSpec) mavenBuildTool {
	tool := mavenBuildTool{
		version: toolSpec.version,
		spec:    toolSpec,
	}

	return tool
}

func (bt mavenBuildTool) archiveFile() string {
	return fmt.Sprintf("apache-maven-%s-bin.tar.gz", bt.version)
}

func (bt mavenBuildTool) downloadURL() string {
	return fmt.Sprintf(
		"%s/maven-%s/%s/binaries/%s",
		mavenDistMirror,
		bt.majorVersion(),
		bt.version,
		bt.archiveFile(),
	)
}

func (bt mavenBuildTool) majorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt mavenBuildTool) installDir() string {
	return filepath.Join(plumbing.ToolsDir(), "maven")
}

func (bt mavenBuildTool) mavenDir() string {
	return filepath.Join(bt.installDir(), fmt.Sprintf("apache-maven-%s", bt.version))
}

func (bt mavenBuildTool) setup(ctx context.Context) error {
	mavenDir := bt.mavenDir()
	cmdPath := fmt.Sprintf("%s/bin", mavenDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	log.Infof(ctx, "Setting PATH to %s", newPath)
	os.Setenv("PATH", newPath)

	return nil
}

// TODO, generalize downloader
func (bt mavenBuildTool) install(ctx context.Context) error {
	mavenDir := bt.mavenDir()

	if _, err := os.Stat(mavenDir); err == nil {
		log.Infof(ctx, "Maven v%s located in %s!", bt.version, mavenDir)
	} else {
		log.Infof(ctx, "Will install Maven v%s into %s", bt.version, bt.installDir())
		downloadURL := bt.downloadURL()

		log.Infof(ctx, "Downloading Maven from URL %s...", downloadURL)
		localFile, err := plumbing.DownloadFileWithCache(ctx, http.DefaultClient, downloadURL)
		if err != nil {
			log.Errorf(ctx, "Unable to download: %v", err)
			return err
		}
		err = archiver.Unarchive(localFile, bt.installDir())
		if err != nil {
			log.Errorf(ctx, "Unable to decompress: %v", err)
			return err
		}

	}

	return nil
}
