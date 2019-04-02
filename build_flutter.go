package main

import (
	"fmt"
	"github.com/mholt/archiver"
	"os"
	"strings"
)

//https://archive.apache.org/dist/flutter/flutter-3/3.3.3/binaries/apache-flutter-3.3.3-bin.tar.gz
var FLUTTER_DIST_MIRROR = "https://storage.googleapis.com/flutter_infra/releases/stable/{{.OS}}/flutter_{{.OS}}_v{{.Version}}-stable.{{.Extension}}"

type FlutterBuildTool struct {
	BuildTool
	_version string
}

func NewFlutterBuildTool(toolSpec string) FlutterBuildTool {
	parts := strings.Split(toolSpec, ":")
	version := parts[1]

	tool := FlutterBuildTool{
		_version: version,
	}

	return tool
}

func (bt FlutterBuildTool) DownloadUrl() string {
	opsys := OS()
	arch := Arch()
	extension := "tar.xz"

	if arch == "amd64" {
		arch = "x64"
	}

	if opsys == "darwin" {
		opsys = "macos"
		extension = "zip"
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

	url, _ := TemplateToString(FLUTTER_DIST_MIRROR, data)

	return url
}

func (bt FlutterBuildTool) MajorVersion() string {
	parts := strings.Split(bt._version, ".")
	return parts[0]
}

func (bt FlutterBuildTool) Version() string {
	return bt._version
}

func (bt FlutterBuildTool) FlutterDir() string {
	workspace := LoadWorkspace()
	return fmt.Sprintf("%s/flutter-%s", workspace.BuildRoot(), bt.Version())
}

func (bt FlutterBuildTool) Setup() error {
	flutterDir := bt.FlutterDir()
	cmdPath := fmt.Sprintf("%s/flutter/bin", flutterDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	fmt.Printf("Setting PATH to %s\n", newPath)
	os.Setenv("PATH", newPath)

	return nil
}

// TODO, generalize downloader
func (bt FlutterBuildTool) Install() error {
	flutterDir := bt.FlutterDir()

	if _, err := os.Stat(flutterDir); err == nil {
		fmt.Printf("Flutter v%s located in %s!\n", bt.Version(), flutterDir)
	} else {
		fmt.Printf("Will install Flutter v%s into %s\n", bt.Version(), flutterDir)
		downloadUrl := bt.DownloadUrl()

		fmt.Printf("Downloading Flutter from URL %s...\n", downloadUrl)
		localFile, err := DownloadFileWithCache(downloadUrl)
		if err != nil {
			fmt.Printf("Unable to download: %v\n", err)
			return err
		}
		err = archiver.Unarchive(localFile, flutterDir)
		if err != nil {
			fmt.Printf("Unable to decompress: %v\n", err)
			return err
		}

	}

	return nil
}
