package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var RUST_DIST_MIRROR = "https://static.rust-lang.org/rustup/dist"

type RustBuildTool struct {
	BuildTool
	_version string
}

func NewRustBuildTool(toolSpec string) RustBuildTool {
	parts := strings.Split(toolSpec, ":")
	version := parts[1]

	tool := RustBuildTool{
		_version: version,
	}

	return tool
}

func (bt RustBuildTool) Version() string {
	return bt._version
}

func (bt RustBuildTool) RustDir() string {
	workspace := LoadWorkspace()
	return fmt.Sprintf("%s/rust-%v", workspace.BuildRoot(), bt.Version())
}

func (bt RustBuildTool) Setup() error {
	rustDir := bt.RustDir()
	cmdPath := fmt.Sprintf("%s/bin", rustDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	fmt.Printf("Setting PATH to %s\n", newPath)
	os.Setenv("PATH", newPath)

	os.Setenv("CARGO_HOME", rustDir)
	os.Setenv("RUSTUP_HOME", rustDir)

	return nil
}

func (bt RustBuildTool) Install() error {

	arch := "x86_64"
	operatingSystem := "unknown-linux-gnu"

	workspace := LoadWorkspace()
	buildDir := workspace.BuildRoot()
	rustPath := bt.RustDir()

	if _, err := os.Stat(rustPath); err == nil {
		fmt.Printf("Rust v%s located in %s!\n", bt.Version(), rustPath)
	} else {
		fmt.Printf("Will install Rust v%s into %s\n", bt.Version(), rustPath)
		extension := ""
		installerFile := fmt.Sprintf("rustup-init%s", extension)
		downloadUrl := fmt.Sprintf("%s/%s-%s/%s", RUST_DIST_MIRROR, arch, operatingSystem, installerFile)
		localFile := filepath.Join(buildDir, installerFile)
		fmt.Printf("Downloading from URL %s to local file %s\n", downloadUrl, localFile)
		err := DownloadFile(localFile, downloadUrl)
		if err != nil {
			fmt.Printf("Unable to download: %v\n", err)
			return err
		}

		os.Chmod(localFile, 0700)
		os.Setenv("CARGO_HOME", rustPath)
		os.Setenv("RUSTUP_HOME", rustPath)

		installCmd := fmt.Sprintf("%s -y", localFile)
		ExecToStdout(installCmd, buildDir)
	}

	return nil

}
