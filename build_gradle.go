package main

import (
	"fmt"
	"github.com/johnewart/archiver"
	"os"
	"path/filepath"
	"strings"
)

var GRADLE_DIST_MIRROR = "https://services.gradle.org/distributions/gradle-{{.Version}}-bin.zip"

type GradleBuildTool struct {
	BuildTool
	_version string
}

func NewGradleBuildTool(toolSpec string) GradleBuildTool {
	parts := strings.Split(toolSpec, ":")
	version := parts[1]

	tool := GradleBuildTool{
		_version: version,
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
	parts := strings.Split(bt._version, ".")
	return parts[0]
}

func (bt GradleBuildTool) Version() string {
	return bt._version
}

func (bt GradleBuildTool) GradleDir() string {
	return filepath.Join(bt.InstallDir(), fmt.Sprintf("gradle-%s", bt.Version()))
}

func (bt GradleBuildTool) InstallDir() string {
	return filepath.Join(ToolsDir(), "gradle")
}

func (bt GradleBuildTool) Setup() error {
	workspace := LoadWorkspace()
	gradleDir := bt.GradleDir()
	cmdPath := filepath.Join(gradleDir, "bin")
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	fmt.Printf("Setting PATH to %s\n", newPath)
	os.Setenv("PATH", newPath)

	gradleHome := filepath.Join(workspace.BuildRoot(), "gradle-home", bt.Version())
	os.Setenv("GRADLE_USER_HOME", gradleHome)

	return nil
}

// TODO, generalize downloader
func (bt GradleBuildTool) Install() error {
	gradleDir := bt.GradleDir()
	installDir := bt.InstallDir()

	if _, err := os.Stat(gradleDir); err == nil {
		fmt.Printf("Gradle v%s located in %s!\n", bt.Version(), gradleDir)
	} else {
		fmt.Printf("Will install Gradle v%s into %s\n", bt.Version(), gradleDir)
		downloadUrl := bt.DownloadUrl()

		fmt.Printf("Downloading Gradle from URL %s...\n", downloadUrl)
		localFile, err := DownloadFileWithCache(downloadUrl)
		if err != nil {
			fmt.Printf("Unable to download: %v\n", err)
			return err
		}
		err = archiver.Unarchive(localFile, installDir)
		if err != nil {
			fmt.Printf("Unable to decompress: %v\n", err)
			return err
		}

	}

	return nil
}
