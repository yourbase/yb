package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
)

// TODO add yourbase/release-test testing facilities

const goreleaseDistMirrorTemplate = "https://github.com/goreleaser/goreleaser/releases/download/%s/%s"

type GoReleaserBuildTool struct {
	version string
	spec    BuildToolSpec
}

func NewGoReleaserBuildTool(toolSpec BuildToolSpec) GoReleaserBuildTool {
	tool := GoReleaserBuildTool{
		version: toolSpec.Version,
		spec:    toolSpec,
	}

	return tool
}

func (bt GoReleaserBuildTool) ArchiveFile() string {
	operatingSystem := bt.spec.InstallTarget.OS()
	arch := bt.spec.InstallTarget.Architecture()
	os := "Linux"
	architecture := "x86_64"
	ext := "tar.gz"

	if operatingSystem == runtime.Darwin {
		os = "Darwin"
	} else if operatingSystem == runtime.Windows {
		os = "Windows"
		ext = "zip"
	}

	// TODO support arm64, armv6
	if arch != runtime.Amd64 {
		architecture = "i386"
	}

	return fmt.Sprintf("goreleaser_%s_%s.%s", os, architecture, ext)
}

func (bt GoReleaserBuildTool) DonwloadUrl() (string, error) {
	return fmt.Sprintf(goreleaseDistMirrorTemplate, bt.Version(), bt.ArchiveFile()), nil
}

func (bt GoReleaserBuildTool) Version() string {
	return bt.version
}

func (bt GoReleaserBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "goreleaser", bt.Version())

	if t.PathExists(ctx, installDir) {
		log.Infof("GoReleaser v%s located in %s!", bt.Version(), installDir)
		return installDir, nil
	}
	log.Infof("Will install GoReleaser v%s into %s", bt.Version(), installDir)

	downloadURL, err := bt.DonwloadUrl()
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}

	log.Infof("Downloading from URL %s ...", downloadURL)
	localFile, err := t.DownloadFile(ctx, downloadURL)
	if err != nil {
		log.Errorf("Unable to download: %v", err)
		return "", err
	}

	err = t.Unarchive(ctx, localFile, installDir)
	if err != nil {
		log.Errorf("Unable to decompress: %v", err)
		return "", err
	}

	return installDir, nil
}

func (bt GoReleaserBuildTool) Setup(ctx context.Context, installDir string) error {
	t := bt.spec.InstallTarget

	t.PrependToPath(ctx, installDir)

	return nil
}
