package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
)

//https://dl.google.com/go/go1.11.5.linux-amd64.tar.gz
const golangDistMirrorTemplate = "https://dl.google.com/go"

type GolangBuildTool struct {
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

func (bt GolangBuildTool) DownloadURL(ctx context.Context) (string, error) {
	return fmt.Sprintf(
		"%s/%s",
		golangDistMirrorTemplate,
		bt.ArchiveFile(),
	), nil
}

func (bt GolangBuildTool) MajorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt GolangBuildTool) Version() string {
	return bt.version
}

// TODO: handle multiple packages, for now this is ok
func (bt GolangBuildTool) Setup(ctx context.Context, golangDir string) error {
	t := bt.spec.InstallTarget

	goPath := t.ToolsDir(ctx)
	pkgPath := bt.spec.PackageDir

	var goPathElements = []string{goPath, pkgPath}
	goPathVar := strings.Join(goPathElements, ":")

	cmdPath := filepath.Join(golangDir, "bin")
	t.PrependToPath(ctx, cmdPath)
	for _, pathElement := range goPathElements {
		if pathElement != "" {
			pathBinDir := filepath.Join(pathElement, "bin")
			t.PrependToPath(ctx, pathBinDir)
		}
	}

	log.Infof("Setting GOROOT to %s", golangDir)
	t.SetEnv("GOROOT", golangDir)
	log.Infof("Setting GOPATH to %s", goPath)
	t.SetEnv("GOPATH", goPathVar)

	return nil
}

// TODO, generalize downloader
func (bt GolangBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "go", bt.Version())
	golangDir := filepath.Join(installDir, "go")

	if t.PathExists(ctx, golangDir) {
		log.Infof("Golang v%s located in %s!", bt.Version(), golangDir)
		return golangDir, nil
	}
	log.Infof("Will install Golang v%s into %s", bt.Version(), golangDir)
	downloadURL, err := bt.DownloadURL(ctx)
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}

	log.Infof("Downloading from URL %s ...", downloadURL)
	localFile, err := t.DownloadFile(ctx, downloadURL)

	err = t.Unarchive(ctx, localFile, installDir)
	if err != nil {
		log.Errorf("Unable to decompress: %v", err)
		return "", err
	}

	return golangDir, nil
}
