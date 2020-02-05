package buildpacks

import (
	"fmt"
	"github.com/yourbase/yb/runtime"
	"os"
	"path/filepath"
	"strings"

	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/types"
)

// https://archive.apache.org/dist/flutter/flutter-3/3.3.3/binaries/apache-flutter-3.3.3-bin.tar.gz
var FLUTTER_DIST_MIRROR = "https://storage.googleapis.com/flutter_infra/releases/{{.Channel}}/{{.OS}}/flutter_{{.OS}}_v{{.Version}}-{{.Channel}}.{{.Extension}}"

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
	channel := "stable"

	if arch == "amd64" {
		arch = "x64"
	}

	if opsys == "darwin" {
		opsys = "macos"
		extension = "zip"
	}

	version := bt.Version()
	parts := strings.Split(version, "_")
	if len(parts) > 2 {
		version = parts[0]
	} else if len(parts) == 2 {
		version = parts[0]
		channel = parts[1]
	}

	data := struct {
		Channel   string
		OS        string
		Arch      string
		Version   string
		Extension string
	}{
		channel,
		opsys,
		arch,
		version,
		extension,
	}

	url, _ := TemplateToString(FLUTTER_DIST_MIRROR, data)

	return url
}

// TODO: Add Channel method?

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
	log.Infof("Setting PATH to %s", newPath)
	runtime.SetEnv("PATH", newPath)

	return nil
}

// TODO, generalize downloader
func (bt FlutterBuildTool) Install() error {
	flutterDir := bt.FlutterDir()
	installDir := bt.InstallDir()

	if _, err := os.Stat(flutterDir); err == nil {
		log.Infof("Flutter v%s located in %s!", bt.Version(), flutterDir)
	} else {
		log.Infof("Will install Flutter v%s into %s", bt.Version(), flutterDir)
		downloadUrl := bt.DownloadUrl()

		log.Infof("Downloading Flutter from URL %s...", downloadUrl)
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

		//RemoveWritePermissionRecursively(installDir)
	}

	return nil
}
