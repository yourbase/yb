package buildpacks

import (
	"fmt"
	"github.com/yourbase/yb/runtime"
	"path/filepath"
	"strings"

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
	operatingSystem := bt.spec.InstallTarget.OS()
	arch := bt.spec.InstallTarget.Architecture()
	os := "linux"
	architecture := "amd64"

	if operatingSystem == runtime.Linux {
		os = "linux"
	} else {
		os = "darwin"
	}

	if arch == runtime.Amd64 {
		architecture = "amd64"
	}

	return fmt.Sprintf("go%s.%s-%s.tar.gz", bt.Version(), os, architecture)
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
	t := bt.spec.InstallTarget

	golangDir := bt.GolangDir()
	goPath := bt.spec.PackageCacheDir
	pkgPath := bt.spec.PackageDir

	var goPathElements = []string{goPath, pkgPath}
	goPathVar := strings.Join(goPathElements, ":")

	cmdPath := filepath.Join(golangDir, "bin")
	t.PrependToPath(cmdPath)
	for _, pathElement := range goPathElements {
		pathBinDir := filepath.Join(pathElement, "bin")
		t.PrependToPath(pathBinDir)
	}

	log.Infof("Setting GOROOT to %s", golangDir)
	t.SetEnv("GOROOT", golangDir)
	log.Infof("Setting GOPATH to %s", goPath)
	t.SetEnv("GOPATH", goPathVar)

	return nil
}

// TODO, generalize downloader
func (bt GolangBuildTool) Install() error {
	t := bt.spec.InstallTarget

	installDir := bt.InstallDir()
	golangDir := bt.GolangDir()

	if t.PathExists(golangDir) {
		log.Infof("Golang v%s located in %s!", bt.Version(), golangDir)
	} else {
		log.Infof("Will install Golang v%s into %s", bt.Version(), golangDir)
		downloadUrl := bt.DownloadUrl()

		log.Infof("Downloading from URL %s ...", downloadUrl)
		localFile, err := t.DownloadFile(downloadUrl)

		err = t.Unarchive(localFile, installDir)
		if err != nil {
			log.Errorf("Unable to decompress: %v", err)
			return err
		}
	}

	return nil
}
