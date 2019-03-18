package main

import (
	"fmt"
	"github.com/mholt/archiver"
	"os"
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
	workspace := LoadWorkspace()
	golangDir := bt.GolangDir()
	goPath := workspace.BuildRoot()

	for _, pkg := range workspace.PackageList() {
		//pkgPath := filepath.Join(workspace.Path, pkg)
		goPath = fmt.Sprintf("%s:%s", goPath, pkg)
	}

	cmdPath := fmt.Sprintf("%s/bin", golangDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	fmt.Printf("Setting PATH to %s\n", newPath)
	os.Setenv("PATH", newPath)
	fmt.Printf("Setting GOROOT to %s\n", golangDir)
	os.Setenv("GOROOT", golangDir)
	fmt.Printf("Setting GOPATH to %s\n", goPath)
	os.Setenv("GOPATH", goPath)

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
		downloadUrl := bt.DownloadUrl()

		fmt.Printf("Downloading from URL %s ...\n", downloadUrl)
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
