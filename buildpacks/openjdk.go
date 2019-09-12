package buildpacks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnewart/archiver"

	. "github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/types"
)

//https://download.java.net/java/GA/jdk11/9/GPL/openjdk-11.0.2_linux-x64_bin.tar.gz
//var OPENJDK_DIST_MIRROR = "https://download.java.net/java/GA"

//https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u202-b08/OpenJDK8U-jdk_x64_mac_hotspot_8u202b08.tar.gz
var OPENJDK_DIST_MIRROR = "https://github.com/AdoptOpenJDK/openjdk{{.MajorVersion}}-binaries/releases/download/jdk{{.MajorVersion}}u{{.MinorVersion}}-b{{.PatchVersion}}/OpenJDK{{.MajorVersion}}U-jdk_{{.Arch}}_{{.OS}}_hotspot_{{.MajorVersion}}u{{.MinorVersion}}b{{.PatchVersion}}.{{.Extension}}"

type JavaBuildTool struct {
	BuildTool
	version string
	spec    BuildToolSpec
}

func NewJavaBuildTool(toolSpec BuildToolSpec) JavaBuildTool {
	tool := JavaBuildTool{
		version: toolSpec.Version,
		spec:    toolSpec,
	}

	return tool
}

func (bt JavaBuildTool) Version() string {
	return bt.version
}

func (bt JavaBuildTool) MajorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt JavaBuildTool) MinorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[1]
}

func (bt JavaBuildTool) PatchVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[2]
}

func (bt JavaBuildTool) InstallDir() string {
	return filepath.Join(bt.spec.SharedCacheDir, "java")
}

func (bt JavaBuildTool) JavaDir() string {
	opsys := OS()
	basePath := filepath.Join(bt.InstallDir(), fmt.Sprintf("jdk%su%s-b%s", bt.MajorVersion(), bt.MinorVersion(), bt.PatchVersion()))

	if opsys == "darwin" {
		basePath = filepath.Join(basePath, "Contents", "Home")
	}

	return basePath
}

func (bt JavaBuildTool) Setup() error {
	javaDir := bt.JavaDir()
	cmdPath := fmt.Sprintf("%s/bin", javaDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	log.Infof("Setting PATH to %s", newPath)
	os.Setenv("PATH", newPath)
	log.Infof("Setting JAVA_HOME to %s", javaDir)
	os.Setenv("JAVA_HOME", javaDir)

	return nil
}

func (bt JavaBuildTool) DownloadUrl() string {
	arch := "x64"
	extension := "tar.gz"

	operatingSystem := OS()
	if operatingSystem == "darwin" {
		operatingSystem = "mac"
	}

	if operatingSystem == "windows" {
		extension = "zip"
	}

	data := struct {
		OS           string
		Arch         string
		MajorVersion string
		MinorVersion string
		PatchVersion string
		Extension    string
	}{
		operatingSystem,
		arch,
		bt.MajorVersion(),
		bt.MinorVersion(),
		bt.PatchVersion(),
		extension,
	}

	log.Infof("URL params: %s", data)

	url, err := TemplateToString(OPENJDK_DIST_MIRROR, data)

	if err != nil {
		log.Errorf("Error generating download URL: %v", err)
	}

	return url
}

func (bt JavaBuildTool) Install() error {
	javaInstallDir := bt.InstallDir()
	javaPath := bt.JavaDir()

	MkdirAsNeeded(javaInstallDir)

	if _, err := os.Stat(javaPath); err == nil {
		log.Infof("Java v%s located in %s!", bt.Version(), javaPath)
	} else {
		log.Infof("Will install Java v%s into %s", bt.Version(), javaInstallDir)
		downloadUrl := bt.DownloadUrl()

		log.Infof("Downloading from URL %s ", downloadUrl)
		localFile, err := DownloadFileWithCache(downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err
		}
		err = archiver.Unarchive(localFile, javaInstallDir)
		if err != nil {
			log.Errorf("Unable to decompress: %v", err)
			return err
		}

	}

	return nil

}
