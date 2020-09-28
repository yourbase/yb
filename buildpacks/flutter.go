package buildpacks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnewart/archiver"
	"golang.org/x/mod/semver"

	"github.com/yourbase/yb/plumbing"
	"zombiezen.com/go/log"
)

// Stable channel URL example:
// https://storage.googleapis.com/flutter_infra/releases/stable/linux/flutter_linux_1.17.5-stable.tar.xz
// Beta channel URL example:
// https://storage.googleapis.com/flutter_infra/releases/beta/linux/flutter_linux_1.19.0-4.2.pre-beta.tar.xz
const flutterDistMirrorTemplate = "https://storage.googleapis.com/flutter_infra/releases/{{.Channel}}/{{.OS}}/flutter_{{.OS}}_{{.Version}}-{{.Channel}}.{{.Extension}}"

type flutterBuildTool struct {
	version string
	spec    buildToolSpec
}

func newFlutterBuildTool(spec buildToolSpec) flutterBuildTool {

	tool := flutterBuildTool{
		version: spec.version,
		spec:    spec,
	}

	return tool
}

func (bt flutterBuildTool) downloadURL() string {
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

	version := bt.version
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
		downloadURLVersion(version),
		extension,
	}
	url, _ := plumbing.TemplateToString(flutterDistMirrorTemplate, data)

	return url
}

// TODO: Add Channel method?

func (bt flutterBuildTool) majorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt flutterBuildTool) installDir() string {
	return filepath.Join(bt.spec.packageCacheDir, "flutter", fmt.Sprintf("flutter-%s", bt.version))
}

func (bt flutterBuildTool) flutterDir() string {
	return filepath.Join(bt.installDir(), "flutter")
}

func (bt flutterBuildTool) setup(ctx context.Context) error {
	flutterDir := bt.flutterDir()

	cmdPath := filepath.Join(flutterDir, "bin")
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	log.Infof(ctx, "Setting PATH to %s", newPath)
	os.Setenv("PATH", newPath)

	return nil
}

// TODO, generalize downloader
func (bt flutterBuildTool) install(ctx context.Context) error {
	flutterDir := bt.flutterDir()
	installDir := bt.installDir()

	if _, err := os.Stat(flutterDir); err == nil {
		log.Infof(ctx, "Flutter %s located in %s!", downloadURLVersion(bt.version), flutterDir)
	} else {
		log.Infof(ctx, "Will install Flutter %s into %s", downloadURLVersion(bt.version), flutterDir)
		downloadUrl := bt.downloadURL()

		log.Infof(ctx, "Downloading Flutter from URL %s...", downloadUrl)
		localFile, err := plumbing.DownloadFileWithCache(downloadUrl)
		if err != nil {
			log.Errorf(ctx, "Unable to download: %v", err)
			return err
		}
		err = archiver.Unarchive(localFile, installDir)
		if err != nil {
			log.Errorf(ctx, "Unable to decompress: %v", err)
			return err
		}

		//RemoveWritePermissionRecursively(installDir)
	}

	return nil
}

// Starting with flutter 1.17 the version format changed.
// Adding support for pre version 1.17 with "v" and keep support for no "v"
// - Pre 1.17 version =>  vx.xx.x or vx.xx.x+hotfix.y
//   https://storage.googleapis.com/.../flutter_windows_v1.12.13+hotfix.9-stable.zip
// - 1.17 (and greater?) => 1.17.0 (no "v" in download URL)
//   https://storage.googleapis.com/.../flutter_windows_1.17.0-stable.zip)
//
// Also, yb tacks on a v for customers when we build the URL.
// This function will be backward compatible (tack on "v"), it will support pre 1.17
// version with a "v", and support 1.17 and greater
//
// Note: We are predicting the future since they could require a "v" again if 1.17.0
// was a mistake
//
func downloadURLVersion(version string) string {
	version1170 := "v1.17.0"
	compVersion := version

	// Semver package requires the version to start with "v"
	if !strings.HasPrefix(compVersion, "v") {
		compVersion = fmt.Sprintf("v%s", version)
	}

	// Below 1.17.0 need the "v", >= to 1.17.0, remove the "v"
	if semver.Compare(compVersion, version1170) < 0 {
		version = compVersion // Need the "v"
	} else {
		version = strings.TrimLeft(compVersion, "v")
	}
	if strings.Count(version, "pre") > 0 || strings.Count(version, "dev") > 0 {
		// Beta/dev versions are considered to be newer, even if semver sees it differently
		// Those versions started to pop up after v1.17.0: https://medium.com/flutter/flutter-spring-2020-update-f723d898d7af
		version = strings.TrimLeft(compVersion, "v")
	}

	return version
}
