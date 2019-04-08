package main

import (
	"fmt"
	"github.com/mholt/archiver"
	"os"
	"path/filepath"
	"strings"
)

//https://download.java.net/java/GA/jdk11/9/GPL/openjdk-11.0.2_linux-x64_bin.tar.gz
//var OPENJDK_DIST_MIRROR = "https://download.java.net/java/GA"

//https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u202-b08/OpenJDK8U-jdk_x64_mac_hotspot_8u202b08.tar.gz
var OPENJDK_DIST_MIRROR = "https://github.com/AdoptOpenJDK/openjdk{{.MajorVersion}}-binaries/releases/download/jdk{{.MajorVersion}}u{{.MinorVersion}}-b{{.PatchVersion}}/OpenJDK{{.MajorVersion}}U-jdk_{{.Arch}}_{{.OS}}_hotspot_{{.MajorVersion}}u{{.MinorVersion}}b{{.PatchVersion}}.{{.Extension}}"

type JavaBuildTool struct {
	BuildTool
	_version string
}

func NewJavaBuildTool(toolSpec string) JavaBuildTool {
	parts := strings.Split(toolSpec, ":")
	version := parts[1]

	tool := JavaBuildTool{
		_version: version,
	}

	return tool
}

func (bt JavaBuildTool) Version() string {
	return bt._version
}

func (bt JavaBuildTool) MajorVersion() string {
	parts := strings.Split(bt._version, ".")
	return parts[0]
}

func (bt JavaBuildTool) MinorVersion() string {
	parts := strings.Split(bt._version, ".")
	return parts[1]
}

func (bt JavaBuildTool) PatchVersion() string {
	parts := strings.Split(bt._version, ".")
	return parts[2]
}

func (bt JavaBuildTool) JavaDir() string {
	opsys := OS()
	basePath := filepath.Join(ToolsDir(), "java", fmt.Sprintf("jdk%su%s-b%s", bt.MajorVersion(), bt.MinorVersion(), bt.PatchVersion()))

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
	fmt.Printf("Setting PATH to %s\n", newPath)
	os.Setenv("PATH", newPath)
	fmt.Printf("Setting JAVA_HOME to %s\n", javaDir)
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

	fmt.Printf("URL params: %s\n", data)

	url, err := TemplateToString(OPENJDK_DIST_MIRROR, data)

	if err != nil {
		fmt.Printf("Error generating download URL: %v\n", err)
	}

	return url
}

func (bt JavaBuildTool) Install() error {

	// https://download.java.net/java/GA/jdk11/9/GPL/openjdk-11.0.2_osx-x64_bin.tar.gz
	toolsDir := ToolsDir()
	javaPath := bt.JavaDir()
	javaInstallDir := filepath.Join(toolsDir, "java")

	MkdirAsNeeded(javaInstallDir)

	if _, err := os.Stat(javaPath); err == nil {
		fmt.Printf("Java v%s located in %s!\n", bt.Version(), javaPath)
	} else {
		fmt.Printf("Will install Java v%s into %s\n", bt.Version(), javaInstallDir)
		downloadUrl := bt.DownloadUrl()

		fmt.Printf("Downloading from URL %s \n", downloadUrl)
		localFile, err := DownloadFileWithCache(downloadUrl)
		if err != nil {
			fmt.Printf("Unable to download: %v\n", err)
			return err
		}
		err = archiver.Unarchive(localFile, javaInstallDir)
		if err != nil {
			fmt.Printf("Unable to decompress: %v\n", err)
			return err
		}

	}

	return nil

}
