package main

import (
	"fmt"
	"github.com/mholt/archiver"
	"os"
	"path/filepath"
	"strings"
)

//https://dl.google.com/go/go1.11.5.linux-amd64.tar.gz
var GOLANG_DIST_MIRROR = "https://dl.google.com/go"

type GolangBuildTool struct {
	BuildTool
	_version string
}

func NewGolangBuildTool(toolSpec string) GolangBuildTool {
	parts := strings.Split(toolSpec, ":")
	version := parts[1]

	tool := GolangBuildTool{
		_version: version,
	}

	return tool
}

func (bt GolangBuildTool) ArchiveFile() string {
	operatingSystem := "linux"
	arch := "amd64"
	return fmt.Sprintf("go%s.%s-%s.tar.gz", bt.Version(), operatingSystem, arch)
}

func (bt GolangBuildTool) DownloadUrl() string {
	return fmt.Sprintf(
		"%s/%s",
		GOLANG_DIST_MIRROR,
		bt.ArchiveFile(),
	)
}

func (bt GolangBuildTool) MajorVersion() string {
	parts := strings.Split(bt._version, ".")
	return parts[0]
}

func (bt GolangBuildTool) Version() string {
	return bt._version
}

func (bt GolangBuildTool) GolangDir() string {
	workspace := LoadWorkspace()
	return fmt.Sprintf("%s/go", workspace.BuildRoot())
}

func (bt GolangBuildTool) Setup() error {
	golangDir := bt.GolangDir()
	cmdPath := fmt.Sprintf("%s/bin", golangDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	fmt.Printf("Setting PATH to %s\n", newPath)
	os.Setenv("PATH", newPath)
	os.Setenv("GOROOT", golangDir)

	return nil
}

// TODO, generalize downloader
func (bt GolangBuildTool) Install() error {
	workspace := LoadWorkspace()
	buildDir := workspace.BuildRoot()
	golangDir := bt.GolangDir()

	if _, err := os.Stat(golangDir); err == nil {
		fmt.Printf("Golang v%s located in %s!\n", bt.Version(), golangDir)
	} else {
		fmt.Printf("Will install Golang v%s into %s\n", bt.Version(), golangDir)
		archiveFile := bt.ArchiveFile()
		downloadUrl := bt.DownloadUrl()

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
