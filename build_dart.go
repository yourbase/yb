package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/johnewart/archiver"
)

//https://archive.apache.org/dist/dart/dart-3/3.3.3/binaries/apache-dart-3.3.3-bin.tar.gz
var DART_DIST_MIRROR = "https://storage.googleapis.com/dart-archive/channels/stable/release/{{.Version}}/sdk/dartsdk-{{.OS}}-{{.Arch}}-release.zip"

type DartBuildTool struct {
	BuildTool
	_version string
}

func NewDartBuildTool(toolSpec string) DartBuildTool {
	parts := strings.Split(toolSpec, ":")
	version := parts[1]

	tool := DartBuildTool{
		_version: version,
	}

	return tool
}

func (bt DartBuildTool) DownloadUrl() string {
	opsys := OS()
	arch := Arch()
	extension := "zip"

	if arch == "amd64" {
		arch = "x64"
	}

	if opsys == "darwin" {
		opsys = "macos"
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

	url, _ := TemplateToString(DART_DIST_MIRROR, data)

	return url
}

func (bt DartBuildTool) MajorVersion() string {
	parts := strings.Split(bt._version, ".")
	return parts[0]
}

func (bt DartBuildTool) Version() string {
	return bt._version
}

func (bt DartBuildTool) DartDir() string {
	workspace := LoadWorkspace()
	return fmt.Sprintf("%s/dart-%s", workspace.BuildRoot(), bt.Version())
}

func (bt DartBuildTool) Setup() error {
	dartDir := bt.DartDir()
	cmdPath := fmt.Sprintf("%s/dart/bin", dartDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	fmt.Printf("Setting PATH to %s\n", newPath)
	os.Setenv("PATH", newPath)

	return nil
}

// TODO, generalize downloader
func (bt DartBuildTool) Install() error {
	dartDir := bt.DartDir()

	if _, err := os.Stat(dartDir); err == nil {
		fmt.Printf("Dart v%s located in %s!\n", bt.Version(), dartDir)
	} else {
		fmt.Printf("Will install Dart v%s into %s\n", bt.Version(), dartDir)
		downloadUrl := bt.DownloadUrl()

		fmt.Printf("Downloading Dart from URL %s...\n", downloadUrl)
		localFile, err := DownloadFileWithCache(downloadUrl)
		if err != nil {
			fmt.Printf("Unable to download: %v\n", err)
			return err
		}
		err = archiver.Unarchive(localFile, dartDir)
		if err != nil {
			fmt.Printf("Unable to decompress: %v\n", err)
			return err
		}
	}

	return nil
}
