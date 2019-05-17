package buildpacks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnewart/archiver"
	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
)

//https://dl.google.com/go/go1.11.5.linux-amd64.tar.gz
var GOLANG_DIST_MIRROR = "https://dl.google.com/go"

type GolangBuildTool struct {
	BuildTool
	version string
	spec    BuildToolSpec
}

func NewGolangBuildTool(toolSpec BuildToolSpec) GolangBuildTool {
	tool := GolangBuildTool{
		version: toolSpec.Version,
		spec:    toolSpec,
	}

	return tool
}

func (bt GolangBuildTool) ArchiveFile() string {
	operatingSystem := OS()
	arch := Arch()
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
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt GolangBuildTool) Version() string {
	return bt.version
}

func (bt GolangBuildTool) InstallDir() string {
	return fmt.Sprintf("%s/go/%s", bt.spec.SharedCacheDir, bt.Version())
}

func (bt GolangBuildTool) GolangDir() string {
	return filepath.Join(bt.InstallDir(), "go")
}

func srcParent(dir string) string {
	// For '/a/b/c/src/d' we guess that GOPATH is /a/b/c.
	parts := strings.Split(dir, "src")
	if len(parts) == 0 { return "" }
	return parts[0]
}

// TODO: handle multiple packages, for now this is ok
func (bt GolangBuildTool) Setup() error {
	golangDir := bt.GolangDir()
	goPath := bt.spec.PackageCacheDir
	pkgPath := bt.spec.PackageDir
	if goPathSrcParent := srcParent(pkgPath); goPathSrcParent != "" {
		goPath = fmt.Sprintf("%s:%s", goPathSrcParent, goPath)
	}

	goPath = fmt.Sprintf("%s:%s", goPath, pkgPath)

	cmdPath := filepath.Join(golangDir, "bin")
	pkgPathBinDir := filepath.Join(bt.spec.PackageCacheDir, "bin")
	PrependToPath(cmdPath)
	PrependToPath(pkgPathBinDir)

	fmt.Printf("Setting GOROOT to %s\n", golangDir)
	os.Setenv("GOROOT", golangDir)
	fmt.Printf("Setting GOPATH to %s\n", goPath)
	os.Setenv("GOPATH", goPath)

	return nil
}

// TODO, generalize downloader
func (bt GolangBuildTool) Install() error {
	installDir := bt.InstallDir()
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
		err = archiver.Unarchive(localFile, installDir)
		if err != nil {
			fmt.Printf("Unable to decompress: %v\n", err)
			return err
		}

		fmt.Printf("Making go installation in %s read-only\n", golangDir)
		RemoveWritePermissionRecursively(golangDir)
	}

	return nil
}
