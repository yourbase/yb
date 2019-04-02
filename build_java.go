package main

import (
	"fmt"
	"github.com/mholt/archiver"
	"os"
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
	opsys := OS()
	if opsys == "darwin" {
		return fmt.Sprintf("%s/jdk-%s.jdk/Contents/Home", workspace.BuildRoot(), bt.Version())
	} else {
		return fmt.Sprintf("%s/jdk-%s", workspace.BuildRoot(), bt.Version())
	}
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
	operatingSystem := OS()
	if operatingSystem == "darwin" {
		operatingSystem = "osx"
	}

	// https://download.java.net/java/GA/jdk11/9/GPL/openjdk-11.0.2_osx-x64_bin.tar.gz
	workspace := LoadWorkspace()
	buildDir := workspace.BuildRoot()
	javaPath := bt.JavaDir()

	if _, err := os.Stat(javaPath); err == nil {
		fmt.Printf("Java v%s located in %s!\n", bt.Version(), javaPath)
	} else {
		fmt.Printf("Will install Java v%s into %s\n", bt.Version(), javaPath)
		archiveFile := fmt.Sprintf("openjdk-%s_%s-%s_bin.tar.gz", bt.Version(), operatingSystem, arch)
		downloadUrl := fmt.Sprintf("%s/jdk%s/9/GPL/%s", OPENJDK_DIST_MIRROR, bt.MajorVersion(), archiveFile)

		fmt.Printf("Downloading from URL %s \n", downloadUrl)
		localFile, err := DownloadFileWithCache(downloadUrl)
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
