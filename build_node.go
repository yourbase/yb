package main

import (
	"fmt"
	"github.com/mholt/archiver"
	"os"
	"path/filepath"
	"strings"
)

var NODE_DIST_MIRROR = "https://nodejs.org/dist"

type NodeBuildTool struct {
	BuildTool
	_version string
}

func NewNodeBuildTool(toolSpec string) NodeBuildTool {
	parts := strings.Split(toolSpec, ":")
	version := parts[1]

	tool := NodeBuildTool{
		_version: version,
	}

	return tool
}

func (bt NodeBuildTool) Version() string {
	return bt._version
}
func (bt NodeBuildTool) PackageString() string {
	version := bt.Version()
	arch := "x64"
	osName := "linux"
	return fmt.Sprintf("node-v%s-%s-%s", version, osName, arch)
}
func (bt NodeBuildTool) Install() error {

	workspace := LoadWorkspace()
	buildDir := fmt.Sprintf("%s/build", workspace.Path)
	nodePkgVersion := bt.PackageString()
	cmdPath := fmt.Sprintf("%s/%s", buildDir, nodePkgVersion)

	if _, err := os.Stat(cmdPath); err == nil {
		fmt.Printf("Node v%s located in %s!\n", bt.Version(), cmdPath)
	} else {
		fmt.Printf("Would install Node v%s into %s\n", bt.Version(), buildDir)
		archiveFile := fmt.Sprintf("%s.tar.gz", nodePkgVersion)
		downloadUrl := fmt.Sprintf("%s/v%s/%s", NODE_DIST_MIRROR, bt.Version(), archiveFile)
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

func (bt NodeBuildTool) Setup() error {

	workspace := LoadWorkspace()
	buildDir := fmt.Sprintf("%s/build", workspace.Path)
	cmdPath := fmt.Sprintf("%s/%s", buildDir, bt.PackageString())
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	fmt.Printf("Setting PATH to %s\n", newPath)
	os.Setenv("PATH", newPath)

	return nil
}
