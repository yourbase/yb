package buildpacks

import (
	"context"
	"path/filepath"

	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/types"
)

var ANDROID_NDK_DIST_MIRROR = "https://dl.google.com/android/repository/android-ndk-{{.Version}}-{{.OS}}-{{.Arch}}.zip"

type AndroidNdkBuildTool struct {
	BuildTool
	version string
	spec    BuildToolSpec
}

func NewAndroidNdkBuildTool(toolSpec BuildToolSpec) AndroidNdkBuildTool {
	version := toolSpec.Version

	tool := AndroidNdkBuildTool{
		version: version,
		spec:    toolSpec,
	}

	return tool
}

func (bt AndroidNdkBuildTool) DownloadUrl() string {
	opsys := OS()
	arch := Arch()
	extension := "zip"

	if arch == "amd64" {
		arch = "x86_64"
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

	url, _ := TemplateToString(ANDROID_NDK_DIST_MIRROR, data)

	return url
}

func (bt AndroidNdkBuildTool) Version() string {
	return bt.version
}

func (bt AndroidNdkBuildTool) Setup(ctx context.Context, ndkDir string) error {
	t := bt.spec.InstallTarget

	log.Infof("Setting ANDROID_NDK_HOME to %s", ndkDir)
	t.SetEnv("ANDROID_NDK_HOME", ndkDir)

	return nil
}

func (bt AndroidNdkBuildTool) Install(ctx context.Context) (error, string) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "android-ndk")
	ndkDir := filepath.Join(installDir, "android-ndk-"+bt.Version())

	if t.PathExists(ctx, ndkDir) {
		log.Infof("Android NDK v%s located in %s!", bt.Version(), ndkDir)
	} else {
		log.Infof("Will install Android NDK v%s into %s", bt.Version(), ndkDir)
		downloadUrl := bt.DownloadUrl()

		log.Infof("Downloading Android NDK v%s from URL %s...", bt.Version(), downloadUrl)
		localFile, err := t.DownloadFile(ctx, downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err, ""
		}
		err = t.Unarchive(ctx, localFile, installDir)
		if err != nil {
			log.Errorf("Unable to decompress: %v", err)
			return err, ""
		}

	}

	return nil, ndkDir
}
