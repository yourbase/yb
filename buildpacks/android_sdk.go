package buildpacks

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnewart/archiver"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/types"
)

var LATEST_VERSION = "4333796"
var ANDROID_DIST_MIRROR = "https://dl.google.com/android/repository/sdk-tools-{{.OS}}-{{.Version}}.zip"

type AndroidBuildTool struct {
	types.BuildTool
	version string
	spec    BuildToolSpec
}

func NewAndroidBuildTool(toolSpec BuildToolSpec) AndroidBuildTool {
	version := toolSpec.Version

	if version == "latest" {
		version = LATEST_VERSION
	}

	tool := AndroidBuildTool{
		version: version,
		spec:    toolSpec,
	}

	return tool
}

func (bt AndroidBuildTool) DownloadUrl() string {
	opsys := OS()
	arch := Arch()
	extension := "zip"

	if arch == "amd64" {
		arch = "x64"
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

	url, _ := plumbing.TemplateToString(ANDROID_DIST_MIRROR, data)

	return url
}

func (bt AndroidBuildTool) MajorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt AndroidBuildTool) Version() string {
	return bt.version
}

func (bt AndroidBuildTool) InstallDir() string {
	return filepath.Join(plumbing.ToolsDir(), "android", fmt.Sprintf("android-%s", bt.Version()))
}

func (bt AndroidBuildTool) AndroidDir() string {
	return filepath.Join(bt.InstallDir())
}

func (bt AndroidBuildTool) writeAgreements() error {
	licensesDir := filepath.Join(bt.AndroidDir(), "licenses")
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
		log.Infof("Wrote hash for agreement: %s", agreementFile)
	}
	return nil
}

func (bt AndroidBuildTool) Setup() error {
	androidDir := bt.AndroidDir()

	binPath := fmt.Sprintf("%s/tools/bin", androidDir)
	toolsPath := fmt.Sprintf("%s/tools", androidDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", binPath, currentPath)
	newPath = fmt.Sprintf("%s:%s", toolsPath, newPath)
	log.Infof("Setting PATH to %s", newPath)
	os.Setenv("PATH", newPath)

	//fmt.Printf("Setting ANDROID_HOME to %s\n", androidHomeDir)
	//os.Setenv("ANDROID_HOME", androidHomeDir)

	log.Infof("Setting ANDROID_SDK_ROOT to %s", androidDir)
	os.Setenv("ANDROID_SDK_ROOT", androidDir)
	os.Setenv("ANDROID_HOME", androidDir)

	log.Infof("Writing agreement hashes...")
	if err := bt.writeAgreements(); err != nil {
		return err
	}

	return nil
}

// TODO, generalize downloader
func (bt AndroidBuildTool) Install() error {
	androidDir := bt.AndroidDir()
	installDir := bt.InstallDir()

	if _, err := os.Stat(androidDir); err == nil {
		log.Infof("Android v%s located in %s!", bt.Version(), androidDir)
	} else {
		log.Infof("Will install Android v%s into %s", bt.Version(), androidDir)
		downloadUrl := bt.DownloadUrl()

		log.Infof("Downloading Android from URL %s...", downloadUrl)
		localFile, err := plumbing.DownloadFileWithCache(downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err
		}
		err = archiver.Unarchive(localFile, installDir)
		if err != nil {
			log.Errorf("Unable to decompress: %v", err)
			return err
		}

	}

	return nil
}
