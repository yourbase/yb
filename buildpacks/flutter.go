package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
)

// Stable channel URL example:
// https://storage.googleapis.com/flutter_infra/releases/stable/linux/flutter_linux_1.17.5-stable.tar.xz
// Beta channel URL example:
// https://storage.googleapis.com/flutter_infra/releases/beta/linux/flutter_linux_1.19.0-4.2.pre-beta.tar.xz
const flutterDistMirrorTemplate = "https://storage.googleapis.com/flutter_infra/releases/{{.Channel}}/{{.OS}}/flutter_{{.OS}}_{{.Version}}-{{.Channel}}.{{.Extension}}"

type FlutterBuildTool struct {
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

func (bt FlutterBuildTool) DownloadURL(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	os := t.OS()
	opsys := "linux"
	arch := "x64"
	architecture := t.Architecture()
	extension := "tar.xz"
	channel := "stable"

	if architecture == runtime.I386 {
		arch = "x32"
	}

	if os == runtime.Darwin {
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
		downloadURLVersion(version),
		extension,
	}
	url, err := TemplateToString(flutterDistMirrorTemplate, data)

	return url, err
}

// TODO: Add Channel method?

func (bt FlutterBuildTool) MajorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt FlutterBuildTool) Version() string {
	return bt.version
}

func (bt FlutterBuildTool) Setup(ctx context.Context, flutterDir string) error {
	t := bt.spec.InstallTarget

	t.PrependToPath(ctx, filepath.Join(flutterDir, "bin"))

	return nil
}

func (bt FlutterBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "flutter", "flutter-"+bt.Version())
	flutterDir := filepath.Join(installDir, "flutter")

	if t.PathExists(ctx, flutterDir) {
		log.Infof("Flutter %s located in %s!", downloadURLVersion(bt.Version()), flutterDir)
		return flutterDir, nil
	}
	log.Infof("Will install Flutter %s into %s", downloadURLVersion(bt.Version()), flutterDir)
	downloadURL, err := bt.DownloadURL(ctx)
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}

	log.Infof("Downloading Flutter from URL %s...", downloadURL)
	localFile, err := t.DownloadFile(ctx, downloadURL)
	if err != nil {
		log.Errorf("Unable to download: %v", err)
		return "", err
	}
	err = t.Unarchive(ctx, localFile, installDir)
	if err != nil {
		log.Errorf("Unable to decompress: %v", err)
		return "", err
	}

	return flutterDir, nil
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
