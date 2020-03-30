package buildpacks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
	. "github.com/yourbase/yb/types"
)

var GRADLE_DIST_MIRROR = "https://services.gradle.org/distributions/gradle-{{.Version}}-bin.zip"

type GradleBuildTool struct {
	BuildTool
	version string
	spec    BuildToolSpec
}

func NewGradleBuildTool(toolSpec BuildToolSpec) GradleBuildTool {

	tool := GradleBuildTool{
		version: toolSpec.Version,
		spec:    toolSpec,
	}

	return tool
}

func (bt GradleBuildTool) ArchiveFile() string {
	return fmt.Sprintf("apache-gradle-%s-bin.tar.gz", bt.Version())
}

func (bt GradleBuildTool) DownloadUrl() string {
	data := struct {
		OS        string
		Arch      string
		Version   string
		Extension string
	}{
		OS(),
		Arch(),
		bt.Version(),
		"zip",
	}

	url, _ := TemplateToString(GRADLE_DIST_MIRROR, data)

	return url
}

func (bt GradleBuildTool) MajorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt GradleBuildTool) Version() string {
	return bt.version
}

func (bt GradleBuildTool) GradleDir() string {
	return filepath.Join(bt.InstallDir(), fmt.Sprintf("gradle-%s", bt.Version()))
}

func (bt GradleBuildTool) InstallDir() string {
	return filepath.Join(bt.spec.SharedCacheDir, "gradle")
}

func (bt GradleBuildTool) Setup() error {
	gradleDir := bt.GradleDir()
	gradleHome := filepath.Join(bt.spec.PackageCacheDir, "gradle-home", bt.Version())
	t := bt.spec.InstallTarget

	log.Infof("Setting GRADLE_USER_HOME to %s", gradleHome)
	runtime.SetEnv("GRADLE_USER_HOME", gradleHome)

	t.PrependToPath(filepath.Join(gradleDir, "bin"))

	return nil
}

// TODO, generalize downloader
func (bt GradleBuildTool) Install() error {
	gradleDir := bt.GradleDir()
	installDir := bt.InstallDir()

	if _, err := os.Stat(gradleDir); err == nil {
		log.Infof("Gradle v%s located in %s!", bt.Version(), gradleDir)
	} else {
		log.Infof("Will install Gradle v%s into %s", bt.Version(), gradleDir)
		downloadUrl := bt.DownloadUrl()

		log.Infof("Downloading Gradle from URL %s...", downloadUrl)
		localFile, err := bt.spec.InstallTarget.DownloadFile(downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err
		}
		err = bt.spec.InstallTarget.Unarchive(localFile, installDir)
		if err != nil {
			log.Errorf("Unable to decompress: %v", err)
			return err
		}

	}

	return nil
}
