package main

import (
	"fmt"
	"github.com/mholt/archiver"
	"os"
	"strings"
)

var LATEST_VERSION = "4333796"
var ANDROID_DIST_MIRROR = "https://dl.google.com/android/repository/sdk-tools-{{.OS}}-{{.Version}}.zip"

type AndroidBuildTool struct {
	BuildTool
	_version string
}

func NewAndroidBuildTool(toolSpec string) AndroidBuildTool {
	parts := strings.Split(toolSpec, ":")
	version := parts[1]

	if version == "latest" {
		version = LATEST_VERSION
	}

	tool := AndroidBuildTool{
		_version: version,
	}

	return tool
}

func (bt AndroidBuildTool) DownloadUrl() string {
	opsys := OS()
	arch := Arch()
	extension := "zip"

	if arch == "amd64" {
		arch = "x64"
	}

	version := bt.Version()

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

	url, _ := TemplateToString(ANDROID_DIST_MIRROR, data)

	return url
}

func (bt AndroidBuildTool) MajorVersion() string {
	parts := strings.Split(bt._version, ".")
	return parts[0]
}

func (bt AndroidBuildTool) Version() string {
	return bt._version
}

func (bt AndroidBuildTool) AndroidDir() string {
	workspace := LoadWorkspace()
	return fmt.Sprintf("%s/android-%s", workspace.BuildRoot(), bt.Version())
}

func (bt AndroidBuildTool) Setup() error {
	androidDir := bt.AndroidDir()
	binPath := fmt.Sprintf("%s/tools/bin", androidDir)
	toolsPath := fmt.Sprintf("%s/tools", androidDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", binPath, currentPath)
	newPath = fmt.Sprintf("%s:%s", toolsPath, newPath)
	fmt.Printf("Setting PATH to %s\n", newPath)
	os.Setenv("PATH", newPath)

	fmt.Printf("Setting ANDROID_HOME to %s\n", androidDir)
	os.Setenv("ANDROID_HOME", androidDir)
	os.Setenv("ANDROID_SDK_ROOT", androidDir)

	return nil
}

// TODO, generalize downloader
func (bt AndroidBuildTool) Install() error {
	androidDir := bt.AndroidDir()

	if _, err := os.Stat(androidDir); err == nil {
		fmt.Printf("Android v%s located in %s!\n", bt.Version(), androidDir)
	} else {
		fmt.Printf("Will install Android v%s into %s\n", bt.Version(), androidDir)
		downloadUrl := bt.DownloadUrl()

		fmt.Printf("Downloading Android from URL %s...\n", downloadUrl)
		localFile, err := DownloadFileWithCache(downloadUrl)
		if err != nil {
			fmt.Printf("Unable to download: %v\n", err)
			return err
		}
		err = archiver.Unarchive(localFile, androidDir)
		if err != nil {
			fmt.Printf("Unable to decompress: %v\n", err)
			return err
		}

	}

	return nil
}
