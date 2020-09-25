package buildpacks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnewart/archiver"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
)

//https://dl.google.com/go/go1.11.5.linux-amd64.tar.gz
const golangDistMirror = "https://dl.google.com/go"

type golangBuildTool struct {
	version string
	spec    buildToolSpec
}

func newGolangBuildTool(toolSpec buildToolSpec) golangBuildTool {
	tool := golangBuildTool{
		version: toolSpec.version,
		spec:    toolSpec,
	}

	return tool
}

func (bt golangBuildTool) archiveFile() string {
	operatingSystem := OS()
	arch := Arch()
	return fmt.Sprintf("go%s.%s-%s.tar.gz", bt.version, operatingSystem, arch)
}

func (bt golangBuildTool) downloadURL() string {
	return fmt.Sprintf(
		"%s/%s",
		golangDistMirror,
		bt.archiveFile(),
	)
}

func (bt golangBuildTool) majorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt golangBuildTool) installDir() string {
	return fmt.Sprintf("%s/go/%s", bt.spec.sharedCacheDir, bt.version)
}

func (bt golangBuildTool) golangDir() string {
	return filepath.Join(bt.installDir(), "go")
}

// TODO: handle multiple packages, for now this is ok
func (bt golangBuildTool) setup(ctx context.Context) error {
	golangDir := bt.golangDir()
	goPath := bt.spec.packageCacheDir
	pkgPath := bt.spec.packageDir

	var goPathElements = []string{goPath, pkgPath}
	goPathVar := strings.Join(goPathElements, ":")

	cmdPath := filepath.Join(golangDir, "bin")
	plumbing.PrependToPath(cmdPath)
	for _, pathElement := range goPathElements {
		pathBinDir := filepath.Join(pathElement, "bin")
		plumbing.PrependToPath(pathBinDir)
	}

	log.Infof("Setting GOROOT to %s", golangDir)
	os.Setenv("GOROOT", golangDir)
	log.Infof("Setting GOPATH to %s", goPath)
	os.Setenv("GOPATH", goPathVar)

	return nil
}

// TODO, generalize downloader
func (bt golangBuildTool) install(ctx context.Context) error {
	installDir := bt.installDir()
	golangDir := bt.golangDir()

	if _, err := os.Stat(golangDir); err == nil {
		log.Infof("Golang v%s located in %s!", bt.version, golangDir)
	} else {
		log.Infof("Will install Golang v%s into %s", bt.version, golangDir)
		downloadUrl := bt.downloadURL()

		log.Infof("Downloading from URL %s ...", downloadUrl)
		localFile, err := plumbing.DownloadFileWithCache(downloadUrl)
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
		plumbing.RemoveWritePermissionRecursively(golangDir)
	}

	return nil
}
