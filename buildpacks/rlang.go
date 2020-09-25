package buildpacks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnewart/archiver"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/types"
)

var RLANG_DIST_MIRROR = "https://cloud.r-project.org/src/base"

type RLangBuildTool struct {
	types.BuildTool
	version string
	spec    BuildToolSpec
}

func NewRLangBuildTool(toolSpec BuildToolSpec) RLangBuildTool {
	tool := RLangBuildTool{
		version: toolSpec.Version,
		spec:    toolSpec,
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
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt RLangBuildTool) Version() string {
	return bt.version
}

func (bt RLangBuildTool) InstallDir() string {
	return filepath.Join(bt.spec.SharedCacheDir, "R")
}

func (bt RLangBuildTool) RLangDir() string {
	return filepath.Join(bt.InstallDir(), fmt.Sprintf("R-%s", bt.Version()))
}

func (bt RLangBuildTool) Setup() error {
	rlangDir := bt.RLangDir()

	cmdPath := fmt.Sprintf("%s/bin", rlangDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	log.Infof("Setting PATH to %s", newPath)
	os.Setenv("PATH", newPath)

	return nil
}

// TODO, generalize downloader
func (bt RLangBuildTool) Install() error {
	installDir := bt.InstallDir()
	rlangDir := bt.RLangDir()

	if _, err := os.Stat(rlangDir); err == nil {
		log.Infof("R v%s located in %s!", bt.Version(), rlangDir)
	} else {
		log.Infof("Will install R v%s into %s", bt.Version(), installDir)
		downloadUrl := bt.DownloadUrl()

		log.Infof("Downloading from URL %s ...", downloadUrl)
		localFile, err := plumbing.DownloadFileWithCache(downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err
		}

		tmpDir := filepath.Join(installDir, "src")
		srcDir := filepath.Join(tmpDir, fmt.Sprintf("R-%s", bt.Version()))

		if !plumbing.DirectoryExists(srcDir) {
			err = archiver.Unarchive(localFile, tmpDir)
			if err != nil {
				log.Errorf("Unable to decompress: %v", err)
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
