package main

import (
	"fmt"
	"github.com/mholt/archiver"
	"os"
	"strings"
)

//https://archive.apache.org/dist/maven/maven-3/3.3.3/binaries/apache-maven-3.3.3-bin.tar.gz
var MAVEN_DIST_MIRROR = "https://archive.apache.org/dist/maven/"

type MavenBuildTool struct {
	BuildTool
	_version string
}

func NewMavenBuildTool(toolSpec string) MavenBuildTool {
	parts := strings.Split(toolSpec, ":")
	version := parts[1]

	tool := MavenBuildTool{
		_version: version,
	}

	return tool
}

func (bt MavenBuildTool) ArchiveFile() string {
	return fmt.Sprintf("apache-maven-%s-bin.tar.gz", bt.Version())
}

func (bt MavenBuildTool) DownloadUrl() string {
	return fmt.Sprintf(
		"%s/maven-%s/%s/binaries/%s",
		MAVEN_DIST_MIRROR,
		bt.MajorVersion(),
		bt.Version(),
		bt.ArchiveFile(),
	)
}

func (bt MavenBuildTool) MajorVersion() string {
	parts := strings.Split(bt._version, ".")
	return parts[0]
}

func (bt MavenBuildTool) Version() string {
	return bt._version
}

func (bt MavenBuildTool) MavenDir() string {
	workspace := LoadWorkspace()
	return fmt.Sprintf("%s/apache-maven-%s", workspace.BuildRoot(), bt.Version())
}

func (bt MavenBuildTool) Setup() error {
	mavenDir := bt.MavenDir()
	cmdPath := fmt.Sprintf("%s/bin", mavenDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	fmt.Printf("Setting PATH to %s\n", newPath)
	os.Setenv("PATH", newPath)

	return nil
}

// TODO, generalize downloader
func (bt MavenBuildTool) Install() error {
	workspace := LoadWorkspace()
	buildDir := workspace.BuildRoot()
	mavenDir := bt.MavenDir()

	if _, err := os.Stat(mavenDir); err == nil {
		fmt.Printf("Maven v%s located in %s!\n", bt.Version(), mavenDir)
	} else {
		fmt.Printf("Will install Maven v%s into %s\n", bt.Version(), mavenDir)
		downloadUrl := bt.DownloadUrl()

		fmt.Printf("Downloading Maven from URL %s...\n", downloadUrl)
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
