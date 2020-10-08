package buildpacks

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnewart/archiver"
	"github.com/yourbase/yb/plumbing"
	"zombiezen.com/go/log"
)

const rLangDistMirror = "https://cloud.r-project.org/src/base"

type rLangBuildTool struct {
	version string
	spec    buildToolSpec
}

func newRLangBuildTool(toolSpec buildToolSpec) rLangBuildTool {
	tool := rLangBuildTool{
		version: toolSpec.version,
		spec:    toolSpec,
	}

	return tool
}

func (bt rLangBuildTool) archiveFile() string {
	return fmt.Sprintf("R-%s.tar.gz", bt.version)
}

func (bt rLangBuildTool) downloadURL() string {
	return fmt.Sprintf(
		"%s/R-%s/%s",
		rLangDistMirror,
		bt.majorVersion(),
		bt.archiveFile(),
	)
}

func (bt rLangBuildTool) majorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt rLangBuildTool) installDir() string {
	return filepath.Join(bt.spec.cacheDir, "R")
}

func (bt rLangBuildTool) rLangDir() string {
	return filepath.Join(bt.installDir(), fmt.Sprintf("R-%s", bt.version))
}

func (bt rLangBuildTool) setup(ctx context.Context) error {
	rlangDir := bt.rLangDir()

	cmdPath := fmt.Sprintf("%s/bin", rlangDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	log.Infof(ctx, "Setting PATH to %s", newPath)
	os.Setenv("PATH", newPath)

	return nil
}

// TODO, generalize downloader
func (bt rLangBuildTool) install(ctx context.Context) error {
	installDir := bt.installDir()
	rlangDir := bt.rLangDir()

	if _, err := os.Stat(rlangDir); err == nil {
		log.Infof(ctx, "R v%s located in %s!", bt.version, rlangDir)
	} else {
		log.Infof(ctx, "Will install R v%s into %s", bt.version, installDir)
		downloadURL := bt.downloadURL()

		log.Infof(ctx, "Downloading from URL %s ...", downloadURL)
		localFile, err := plumbing.DownloadFileWithCache(ctx, http.DefaultClient, bt.spec.dataDirs, downloadURL)
		if err != nil {
			log.Errorf(ctx, "Unable to download: %v", err)
			return err
		}

		tmpDir := filepath.Join(installDir, "src")
		srcDir := filepath.Join(tmpDir, fmt.Sprintf("R-%s", bt.version))

		if !plumbing.DirectoryExists(srcDir) {
			err = archiver.Unarchive(localFile, tmpDir)
			if err != nil {
				log.Errorf(ctx, "Unable to decompress: %v", err)
				return err
			}
		}

		if err := os.MkdirAll(rlangDir, 0777); err != nil {
			return fmt.Errorf("install R: %w", err)
		}
		configCmd := fmt.Sprintf("./configure --with-x=no --prefix=%s", rlangDir)
		plumbing.ExecToStdout(configCmd, srcDir)

		plumbing.ExecToStdout("make", srcDir)
		plumbing.ExecToStdout("make install", srcDir)
	}

	return nil
}
