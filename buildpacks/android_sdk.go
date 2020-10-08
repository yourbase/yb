package buildpacks

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnewart/archiver"
	"github.com/yourbase/yb/plumbing"
	"zombiezen.com/go/log"
)

const (
	latestAndroidVersion = "4333796"
	androidDistMirror    = "https://dl.google.com/android/repository/sdk-tools-{{.OS}}-{{.Version}}.zip"
)

type androidBuildTool struct {
	version string
	spec    buildToolSpec
}

func newAndroidBuildTool(toolSpec buildToolSpec) androidBuildTool {
	version := toolSpec.version

	if version == "latest" {
		version = latestAndroidVersion
	}

	tool := androidBuildTool{
		version: version,
		spec:    toolSpec,
	}

	return tool
}

func (bt androidBuildTool) downloadURL() string {
	opsys := OS()
	arch := Arch()
	extension := "zip"

	if arch == "amd64" {
		arch = "x64"
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

	url, _ := plumbing.TemplateToString(androidDistMirror, data)

	return url
}

func (bt androidBuildTool) majorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt androidBuildTool) installDir() string {
	return filepath.Join(bt.spec.cacheDir, "android", fmt.Sprintf("android-%s", bt.version))
}

func (bt androidBuildTool) androidDir() string {
	return filepath.Join(bt.installDir())
}

func (bt androidBuildTool) writeAgreements(ctx context.Context) error {
	licensesDir := filepath.Join(bt.androidDir(), "licenses")
	if err := os.MkdirAll(licensesDir, 0777); err != nil {
		return fmt.Errorf("create agreement files: %w", err)
	}

	agreements := map[string]string{
		"android-googletv-license":      "601085b94cd77f0b54ff86406957099ebe79c4d6",
		"android-sdk-license":           "24333f8a63b6825ea9c5514f83c2829b004d1fee",
		"android-sdk-preview-license":   "84831b9409646a918e30573bab4c9c91346d8abd",
		"google-gdk-license":            "33b6a2b64607f11b759f320ef9dff4ae5c47d97a",
		"intel-android-extra-license":   "d975f751698a77b662f1254ddbeed3901e976f5a",
		"mips-android-sysimage-license": "e9acab5b5fbb560a72cfaecce8946896ff6aab9d",
	}
	for filename, hash := range agreements {
		agreementFile := filepath.Join(licensesDir, filename)
		if err := ioutil.WriteFile(agreementFile, []byte(hash), 0666); err != nil {
			return fmt.Errorf("create agreement file %s: %w", filename, err)
		}
		log.Infof(ctx, "Wrote hash for agreement: %s", agreementFile)
	}
	return nil
}

func (bt androidBuildTool) setup(ctx context.Context) error {
	androidDir := bt.androidDir()

	binPath := fmt.Sprintf("%s/tools/bin", androidDir)
	toolsPath := fmt.Sprintf("%s/tools", androidDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", binPath, currentPath)
	newPath = fmt.Sprintf("%s:%s", toolsPath, newPath)
	log.Infof(ctx, "Setting PATH to %s", newPath)
	os.Setenv("PATH", newPath)

	//fmt.Printf("Setting ANDROID_HOME to %s\n", androidHomeDir)
	//os.Setenv("ANDROID_HOME", androidHomeDir)

	log.Infof(ctx, "Setting ANDROID_SDK_ROOT to %s", androidDir)
	os.Setenv("ANDROID_SDK_ROOT", androidDir)
	os.Setenv("ANDROID_HOME", androidDir)

	log.Infof(ctx, "Writing agreement hashes...")
	if err := bt.writeAgreements(ctx); err != nil {
		return err
	}

	return nil
}

// TODO, generalize downloader
func (bt androidBuildTool) install(ctx context.Context) error {
	androidDir := bt.androidDir()
	installDir := bt.installDir()

	if _, err := os.Stat(androidDir); err == nil {
		log.Infof(ctx, "Android v%s located in %s!", bt.version, androidDir)
	} else {
		log.Infof(ctx, "Will install Android v%s into %s", bt.version, androidDir)
		downloadURL := bt.downloadURL()

		log.Infof(ctx, "Downloading Android from URL %s...", downloadURL)
		localFile, err := plumbing.DownloadFileWithCache(ctx, http.DefaultClient, downloadURL)
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
