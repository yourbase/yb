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

const gradleDistMirror = "https://services.gradle.org/distributions/gradle-{{.Version}}-bin.zip"

type gradleBuildTool struct {
	version string
	spec    buildToolSpec
}

func newGradleBuildTool(toolSpec buildToolSpec) gradleBuildTool {

	tool := gradleBuildTool{
		version: toolSpec.version,
		spec:    toolSpec,
	}

	return tool
}

func (bt gradleBuildTool) archiveFile() string {
	return fmt.Sprintf("apache-gradle-%s-bin.tar.gz", bt.version)
}

func (bt gradleBuildTool) downloadURL() string {
	data := struct {
		OS        string
		Arch      string
		Version   string
		Extension string
	}{
		OS(),
		Arch(),
		bt.version,
		"zip",
	}

	url, _ := plumbing.TemplateToString(gradleDistMirror, data)

	return url
}

func (bt gradleBuildTool) majorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt gradleBuildTool) gradleDir() string {
	return filepath.Join(bt.installDir(), fmt.Sprintf("gradle-%s", bt.version))
}

func (bt gradleBuildTool) installDir() string {
	return filepath.Join(bt.spec.cacheDir, "gradle")
}

func (bt gradleBuildTool) setup(ctx context.Context) error {
	gradleDir := bt.gradleDir()
	gradleHome := filepath.Join(bt.spec.cacheDir, "gradle-home", bt.version)

	cmdPath := filepath.Join(gradleDir, "bin")
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	log.Infof(ctx, "Setting PATH to %s", newPath)
	os.Setenv("PATH", newPath)

	log.Infof(ctx, "Setting GRADLE_USER_HOME to %s", gradleHome)
	os.Setenv("GRADLE_USER_HOME", gradleHome)

	return nil
}

// TODO, generalize downloader
func (bt gradleBuildTool) install(ctx context.Context) error {
	gradleDir := bt.gradleDir()
	installDir := bt.installDir()

	if _, err := os.Stat(gradleDir); err == nil {
		log.Infof(ctx, "Gradle v%s located in %s!", bt.version, gradleDir)
	} else {
		log.Infof(ctx, "Will install Gradle v%s into %s", bt.version, gradleDir)
		downloadURL := bt.downloadURL()

		log.Infof(ctx, "Downloading Gradle from URL %s...", downloadURL)
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
