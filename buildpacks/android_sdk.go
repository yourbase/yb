package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
)

const (
	latestVersion                = "4333796"
	androidSDKDistMirrorTemplate = "https://dl.google.com/android/repository/sdk-tools-{{.OS}}-{{.Version}}.zip"
)

type AndroidBuildTool struct {
	version string
	spec    BuildToolSpec
}

func NewAndroidBuildTool(toolSpec BuildToolSpec) AndroidBuildTool {
	version := toolSpec.Version

	if version == "latest" {
		version = latestVersion
	}

	tool := AndroidBuildTool{
		version: version,
		spec:    toolSpec,
	}

	return tool
}

func (bt AndroidBuildTool) DownloadURL(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	os := t.OS()
	opsys := "linux"
	arch := "x64"
	architecture := t.Architecture()
	extension := "zip"

	if architecture == runtime.I386 {
		arch = "x32"
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

	url, err := TemplateToString(androidSDKDistMirrorTemplate, data)
	return url, err
}

func (bt AndroidBuildTool) MajorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt AndroidBuildTool) Version() string {
	return bt.version
}

func (bt AndroidBuildTool) writeAgreements(ctx context.Context, androidDir string) bool {
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
		err := t.WriteFileContents(ctx, hash, agreementFile)
		if err != nil {
			log.Errorf("Can't create agreement file %s: %v", agreementFile, err)
			return false
		}
		log.Infof("Wrote hash for agreement: %s", agreementFile)
	}

	return true

}

func (bt AndroidBuildTool) Setup(ctx context.Context, androidDir string) error {
	t := bt.spec.InstallTarget

	log.Infof("Setting ANDROID_SDK_ROOT to %s", androidDir)
	t.SetEnv("ANDROID_SDK_ROOT", androidDir)
	t.SetEnv("ANDROID_HOME", filepath.Join(t.ToolOutputSharedDir(ctx), "android", bt.Version()))

	log.Infof("Writing agreement hashes...")
	if !bt.writeAgreements(ctx, androidDir) {
		return fmt.Errorf("auto write the agreements")
	}

	t.PrependToPath(ctx, filepath.Join(androidDir, "tools"))
	t.PrependToPath(ctx, filepath.Join(androidDir, "tools", "bin"))

	return nil
}

// TODO, generalize downloader
func (bt AndroidBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "android")
	androidDir := filepath.Join(installDir, "android-"+bt.Version())

	if t.PathExists(ctx, androidDir) {
		log.Infof("Android v%s located in %s!", bt.Version(), androidDir)
		return androidDir, nil
	}
	log.Infof("Will install Android v%s into %s", bt.Version(), androidDir)
	downloadURL, err := bt.DownloadURL(ctx)
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}

	log.Infof("Downloading Android from URL %s...", downloadURL)
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

	return androidDir, nil
}
