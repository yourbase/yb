package buildpacks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnewart/archiver"
	. "github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
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

// TODO: handle multiple packages, for now this is ok
func (bt GolangBuildTool) Setup() error {
	golangDir := bt.GolangDir()
	goPath := bt.spec.PackageCacheDir
	pkgPath := bt.spec.PackageDir

	var goPathElements = []string{goPath, pkgPath}
	goPathVar := strings.Join(goPathElements, ":")

	cmdPath := filepath.Join(golangDir, "bin")
	PrependToPath(cmdPath)
	for _, pathElement := range goPathElements {
		pathBinDir := filepath.Join(pathElement, "bin")
		PrependToPath(pathBinDir)
	}

	log.Infof("Setting GOROOT to %s", golangDir)
	os.Setenv("GOROOT", golangDir)
	log.Infof("Setting GOPATH to %s", goPath)
	os.Setenv("GOPATH", goPathVar)

	return nil
}

// TODO, generalize downloader
func (bt GolangBuildTool) Install() error {
	installDir := bt.InstallDir()
	golangDir := bt.GolangDir()

	if _, err := os.Stat(golangDir); err == nil {
		log.Infof("Golang v%s located in %s!", bt.Version(), golangDir)
	} else {
		log.Infof("Will install Golang v%s into %s", bt.Version(), golangDir)
		downloadUrl := bt.DownloadUrl()

		log.Infof("Downloading from URL %s ...", downloadUrl)
		localFile, err := DownloadFileWithCache(downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err
		}
		err = archiver.Unarchive(localFile, installDir)
		if err != nil {
			log.Errorf("Unable to decompress: %v", err)
			return err
		}

		log.Infof("Making go installation in %s read-only", golangDir)
		RemoveWritePermissionRecursively(golangDir)
	}

	return nil
}
