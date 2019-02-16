package main

import (
	"fmt"
	"github.com/mholt/archiver"
	"os"
	"path/filepath"
	"strings"
)

//https://download.java.net/java/GA/jdk11/9/GPL/openjdk-11.0.2_linux-x64_bin.tar.gz
var OPENJDK_DIST_MIRROR = "https://download.java.net/java/GA"

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

func (bt JavaBuildTool) JavaDir() string {
	workspace := LoadWorkspace()
	return fmt.Sprintf("%s/jdk-%s", workspace.BuildRoot(), bt.Version())
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

func (bt JavaBuildTool) Install() error {

	arch := "x64"
	operatingSystem := "linux"

	workspace := LoadWorkspace()
	buildDir := workspace.BuildRoot()
	rustPath := bt.JavaDir()

	if _, err := os.Stat(rustPath); err == nil {
		fmt.Printf("Java v%s located in %s!\n", bt.Version(), rustPath)
	} else {
		fmt.Printf("Will install Java v%s into %s\n", bt.Version(), rustPath)
		archiveFile := fmt.Sprintf("openjdk-%s_%s-%s_bin.tar.gz", bt.Version(), operatingSystem, arch)
		downloadUrl := fmt.Sprintf("%s/jdk%s/9/GPL/%s", OPENJDK_DIST_MIRROR, bt.MajorVersion(), archiveFile)

		localFile := filepath.Join(buildDir, archiveFile)
		fmt.Printf("Downloading from URL %s to local file %s\n", downloadUrl, localFile)
		err := DownloadFile(localFile, downloadUrl)
		if err != nil {
			fmt.Printf("Unable to download: %v\n", err)
			return err
		}
		err = archiver.Unarchive(localFile, buildDir)
		if err != nil {
			fmt.Printf("Unable to decompress: %v\n", err)
			return err
		}

	}

	return nil

}
