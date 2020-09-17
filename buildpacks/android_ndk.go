package buildpacks

import (
	"context"
	"path/filepath"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
)

const androidNDKDistMirrorTemplate = "https://dl.google.com/android/repository/android-ndk-{{.Version}}-{{.OS}}-{{.Arch}}.zip"

type AndroidNdkBuildTool struct {
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

func (bt AndroidNdkBuildTool) DownloadURL(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	os := t.OS()
	opsys := "linux"
	architecture := t.Architecture()
	arch := "x86_64"
	extension := "zip"

	if architecture == runtime.I386 {
		arch = "i386"
	}

	if os == runtime.Darwin {
		opsys = "darwin"
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

	url, err := TemplateToString(androidNDKDistMirrorTemplate, data)

	return url, err
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

func (bt AndroidNdkBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "android-ndk")
	ndkDir := filepath.Join(installDir, "android-ndk-"+bt.Version())

	if t.PathExists(ctx, ndkDir) {
		log.Infof("Android NDK v%s located in %s!", bt.Version(), ndkDir)
		return ndkDir, nil
	}
	log.Infof("Will install Android NDK v%s into %s", bt.Version(), ndkDir)
	downloadURL, err := bt.DownloadURL(ctx)
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}

	log.Infof("Downloading Android NDK v%s from URL %s...", bt.Version(), downloadURL)
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

	return ndkDir, nil
}
