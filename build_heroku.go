package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/johnewart/archiver"
)

//https://archive.apache.org/dist/heroku/heroku-3/3.3.3/binaries/apache-heroku-3.3.3-bin.tar.gz
var HEROKU_DIST_MIRROR = "https://cli-assets.heroku.com/heroku-{{.OS}}-{{.Arch}}.tar.gz"

type HerokuBuildTool struct {
	BuildTool
	_version string
}

func NewHerokuBuildTool(toolSpec string) HerokuBuildTool {
	parts := strings.Split(toolSpec, ":")
	version := parts[1]

	tool := HerokuBuildTool{
		_version: version,
	}

	return tool
}

func (bt HerokuBuildTool) ArchiveFile() string {
	return fmt.Sprintf("apache-heroku-%s-bin.tar.gz", bt.Version())
}

func (bt HerokuBuildTool) DownloadUrl() string {
	opsys := OS()
	arch := Arch()

	if arch == "amd64" {
		arch = "x64"
	}

	data := struct {
		OS   string
		Arch string
	}{
		opsys,
		arch,
	}

	url, _ := TemplateToString(HEROKU_DIST_MIRROR, data)

	return url
}

func (bt HerokuBuildTool) MajorVersion() string {
	parts := strings.Split(bt._version, ".")
	return parts[0]
}

func (bt HerokuBuildTool) Version() string {
	return bt._version
}

func (bt HerokuBuildTool) HerokuDir() string {
	workspace := LoadWorkspace()
	return fmt.Sprintf("%s/heroku-%s", workspace.BuildRoot(), bt.Version())
}

func (bt HerokuBuildTool) Setup() error {
	herokuDir := bt.HerokuDir()
	cmdPath := fmt.Sprintf("%s/heroku/bin", herokuDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	fmt.Printf("Setting PATH to %s\n", newPath)
	os.Setenv("PATH", newPath)

	return nil
}

// TODO, generalize downloader
func (bt HerokuBuildTool) Install() error {
	herokuDir := bt.HerokuDir()

	if _, err := os.Stat(herokuDir); err == nil {
		fmt.Printf("Heroku v%s located in %s!\n", bt.Version(), herokuDir)
	} else {
		fmt.Printf("Will install Heroku v%s into %s\n", bt.Version(), herokuDir)
		downloadUrl := bt.DownloadUrl()

		fmt.Printf("Downloading Heroku from URL %s...\n", downloadUrl)
		localFile, err := DownloadFileWithCache(downloadUrl)
		if err != nil {
			fmt.Printf("Unable to download: %v\n", err)
			return err
		}
		err = archiver.Unarchive(localFile, herokuDir)
		if err != nil {
			fmt.Printf("Unable to decompress: %v\n", err)
			return err
		}

	}

	return nil
}
