package buildpacks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnewart/archiver"

	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
)

//https://archive.apache.org/dist/flutter/flutter-3/3.3.3/binaries/apache-flutter-3.3.3-bin.tar.gz
var FLUTTER_DIST_MIRROR = "https://storage.googleapis.com/flutter_infra/releases/stable/{{.OS}}/flutter_{{.OS}}_v{{.Version}}-stable.{{.Extension}}"

type FlutterBuildTool struct {
	BuildTool
	version string
	spec    BuildToolSpec
}

func NewFlutterBuildTool(spec BuildToolSpec) FlutterBuildTool {

	tool := FlutterBuildTool{
		version: spec.Version,
		spec:    spec,
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
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt FlutterBuildTool) Version() string {
	return bt.version
}

func (bt FlutterBuildTool) InstallDir() string {
	return filepath.Join(bt.spec.PackageCacheDir, "flutter", fmt.Sprintf("flutter-%s", bt.Version()))
}

func (bt FlutterBuildTool) FlutterDir() string {
	return filepath.Join(bt.InstallDir(), "flutter")
}

func (bt FlutterBuildTool) Setup() error {
	flutterDir := bt.FlutterDir()

	cmdPath := filepath.Join(flutterDir, "bin")
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	fmt.Printf("Setting PATH to %s\n", newPath)
	os.Setenv("PATH", newPath)

	return nil
}

// TODO, generalize downloader
func (bt FlutterBuildTool) Install() error {
	flutterDir := bt.FlutterDir()
	installDir := bt.InstallDir()

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
		err = archiver.Unarchive(localFile, installDir)
		if err != nil {
			fmt.Printf("Unable to decompress: %v\n", err)
			return err
		}

		//RemoveWritePermissionRecursively(installDir)
	}

	return nil
}
