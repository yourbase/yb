package buildpacks

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/johnewart/archiver"
	"github.com/yourbase/yb/plumbing"
	"zombiezen.com/go/log"
)

const androidNDKDistMirror = "https://dl.google.com/android/repository/android-ndk-{{.Version}}-{{.OS}}-{{.Arch}}.zip"

type androidNDKBuildTool struct {
	version string
	spec    buildToolSpec
}

func newAndroidNDKBuildTool(toolSpec buildToolSpec) androidNDKBuildTool {
	version := toolSpec.version

	tool := androidNDKBuildTool{
		version: version,
		spec:    toolSpec,
	}

	return tool
}

func (bt androidNDKBuildTool) downloadURL() string {
	opsys := OS()
	arch := Arch()
	extension := "zip"

	if arch == "amd64" {
		arch = "x86_64"
	}

	version := bt.version

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

	url, _ := plumbing.TemplateToString(androidNDKDistMirror, data)

	return url
}

func (bt androidNDKBuildTool) installDir() string {
	return filepath.Join(bt.spec.cacheDir, "android-ndk")
}

func (bt androidNDKBuildTool) ndkDir() string {
	return filepath.Join(bt.installDir(), fmt.Sprintf("android-ndk-%s", bt.version))
}

func (bt androidNDKBuildTool) setup(ctx context.Context) error {
	ndkDir := bt.ndkDir()

	log.Infof(ctx, "Setting ANDROID_NDK_HOME to %s", ndkDir)
	os.Setenv("ANDROID_NDK_HOME", ndkDir)

	return nil
}

// TODO, generalize downloader
func (bt androidNDKBuildTool) install(ctx context.Context) error {
	ndkDir := bt.ndkDir()
	installDir := bt.installDir()

	if _, err := os.Stat(ndkDir); err == nil {
		log.Infof(ctx, "Android NDK v%s located in %s!", bt.version, ndkDir)
	} else {
		log.Infof(ctx, "Will install Android NDK v%s into %s", bt.version, ndkDir)
		downloadURL := bt.downloadURL()

		log.Infof(ctx, "Downloading Android NDK v%s from URL %s...", bt.version, downloadURL)
		localFile, err := plumbing.DownloadFileWithCache(ctx, http.DefaultClient, bt.spec.dataDirs, downloadURL)
		if err != nil {
			log.Errorf(ctx, "Unable to download: %v", err)
			return err
		}
		err = archiver.Unarchive(localFile, installDir)
		if err != nil {
			log.Errorf(ctx, "Unable to decompress: %v", err)
			return err
		}

	}

	return nil
}
