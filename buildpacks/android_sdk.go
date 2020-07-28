package buildpacks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/types"
)

var LATEST_VERSION = "4333796"
var ANDROID_DIST_MIRROR = "https://dl.google.com/android/repository/sdk-tools-{{.OS}}-{{.Version}}.zip"

type AndroidBuildTool struct {
	BuildTool
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

	url, _ := TemplateToString(ANDROID_DIST_MIRROR, data)

	return url
}

func (bt AndroidBuildTool) MajorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt AndroidBuildTool) Version() string {
	return bt.version
}

func (bt AndroidBuildTool) WriteAgreements(ctx context.Context, androidDir string) bool {
	agreements := map[string]string{
		"android-googletv-license":      "601085b94cd77f0b54ff86406957099ebe79c4d6",
		"android-sdk-license":           "24333f8a63b6825ea9c5514f83c2829b004d1fee",
		"android-sdk-preview-license":   "84831b9409646a918e30573bab4c9c91346d8abd",
		"google-gdk-license":            "33b6a2b64607f11b759f320ef9dff4ae5c47d97a",
		"intel-android-extra-license":   "d975f751698a77b662f1254ddbeed3901e976f5a",
		"mips-android-sysimage-license": "e9acab5b5fbb560a72cfaecce8946896ff6aab9d",
	}

	licensesDir := filepath.Join(androidDir, "licenses")
	t := bt.spec.InstallTarget
	t.MkdirAsNeeded(ctx, licensesDir)

	for filename, hash := range agreements {
		agreementFile := filepath.Join(licensesDir, filename)
		// TODO add as a Runtime/Target method: CreateFile
		f, err := os.Create(agreementFile)
		if err != nil {
			log.Errorf("Can't create agreement file %s: %v", agreementFile, err)
			return false
		}

		defer f.Close()
		_, err = f.WriteString(hash)

		if err != nil {
			log.Errorf("Can't write agreement file %s: %v", agreementFile, err)
			return false
		}

		f.Sync()

		log.Infof("Wrote hash for agreement: %s", agreementFile)
	}

	return true

}

func (bt AndroidBuildTool) Setup(ctx context.Context, androidDir string) error {
	t := bt.spec.InstallTarget

	log.Infof("Setting ANDROID_SDK_ROOT to %s", androidDir)
	t.SetEnv("ANDROID_SDK_ROOT", androidDir)
	t.SetEnv("ANDROID_HOME", androidDir)

	log.Infof("Writing agreement hashes...")
	if !bt.WriteAgreements(ctx, androidDir) {
		return fmt.Errorf("Unable to auto write the agreements")
	}

	t.PrependToPath(ctx, filepath.Join(androidDir, "tools"))
	t.PrependToPath(ctx, filepath.Join(androidDir, "tools", "bin"))

	return nil
}

// TODO, generalize downloader
func (bt AndroidBuildTool) Install(ctx context.Context) (error, string) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "android")
	androidDir := filepath.Join(installDir, "android-"+bt.Version())

	if t.PathExists(ctx, androidDir) {
		log.Infof("Android v%s located in %s!", bt.Version(), androidDir)
	} else {
		log.Infof("Will install Android v%s into %s", bt.Version(), androidDir)
		downloadUrl := bt.DownloadUrl()

		log.Infof("Downloading Android from URL %s...", downloadUrl)
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

	return nil, androidDir
}
