package main

import (
	"fmt"
	"github.com/johnewart/archiver"
	"os"
	"path/filepath"
	"strings"
)

var RLANG_DIST_MIRROR = "https://cloud.r-project.org/src/base"

type RLangBuildTool struct {
	BuildTool
	_version string
}

func NewRLangBuildTool(toolSpec string) RLangBuildTool {
	parts := strings.Split(toolSpec, ":")
	version := parts[1]

	tool := RLangBuildTool{
		_version: version,
	}

	return tool
}

func (bt RLangBuildTool) ArchiveFile() string {
	return fmt.Sprintf("R-%s.tar.gz", bt.Version())
}

func (bt RLangBuildTool) DownloadUrl() string {
	return fmt.Sprintf(
		"%s/R-%s/%s",
		RLANG_DIST_MIRROR,
		bt.MajorVersion(),
		bt.ArchiveFile(),
	)
}

func (bt RLangBuildTool) MajorVersion() string {
	parts := strings.Split(bt._version, ".")
	return parts[0]
}

func (bt RLangBuildTool) Version() string {
	return bt._version
}

func (bt RLangBuildTool) RLangDir() string {
	workspace := LoadWorkspace()
	return fmt.Sprintf("%s/R-%s", workspace.BuildRoot(), bt.Version())
}

func (bt RLangBuildTool) Setup() error {
	workspace := LoadWorkspace()
	rlangDir := bt.RLangDir()
	goPath := workspace.BuildRoot()

	for _, pkg := range workspace.PackageList() {
		//pkgPath := filepath.Join(workspace.Path, pkg)
		goPath = fmt.Sprintf("%s:%s", goPath, pkg)
	}

	cmdPath := fmt.Sprintf("%s/bin", rlangDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	fmt.Printf("Setting PATH to %s\n", newPath)
	os.Setenv("PATH", newPath)
	fmt.Printf("Setting GOROOT to %s\n", rlangDir)
	os.Setenv("GOROOT", rlangDir)
	fmt.Printf("Setting GOPATH to %s\n", goPath)
	os.Setenv("GOPATH", goPath)

	return nil
}

// TODO, generalize downloader
func (bt RLangBuildTool) Install() error {
	workspace := LoadWorkspace()
	buildDir := workspace.BuildRoot()
	rlangDir := bt.RLangDir()

	if _, err := os.Stat(rlangDir); err == nil {
		fmt.Printf("R v%s located in %s!\n", bt.Version(), rlangDir)
	} else {
		fmt.Printf("Will install R v%s into %s\n", bt.Version(), rlangDir)
		downloadUrl := bt.DownloadUrl()

		fmt.Printf("Downloading from URL %s ...\n", downloadUrl)
		localFile, err := DownloadFileWithCache(downloadUrl)
		if err != nil {
			fmt.Printf("Unable to download: %v\n", err)
			return err
		}

		tmpDir := filepath.Join(buildDir, "src")
		srcDir := filepath.Join(tmpDir, fmt.Sprintf("R-%s", bt.Version()))

		if !DirectoryExists(srcDir) {
			err = archiver.Unarchive(localFile, tmpDir)
			if err != nil {
				fmt.Printf("Unable to decompress: %v\n", err)
				return err
			}
		}

		configCmd := fmt.Sprintf("./configure --with-x=no --prefix=%s", bt.RLangDir())
		ExecToStdout(configCmd, srcDir)

		ExecToStdout("make", srcDir)
		ExecToStdout("make install", srcDir)
	}

	return nil
}
