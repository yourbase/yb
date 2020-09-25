package buildpacks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/johnewart/archiver"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
)

const nodeDistMirror = "https://nodejs.org/dist"

type nodeBuildTool struct {
	version string
	spec    buildToolSpec
}

func newNodeBuildTool(toolSpec buildToolSpec) nodeBuildTool {
	tool := nodeBuildTool{
		version: toolSpec.version,
		spec:    toolSpec,
	}

	return tool
}

func (bt nodeBuildTool) packageString() string {
	version := bt.version
	arch := Arch()

	if arch == "amd64" {
		arch = "x64"
	}

	osName := OS()

	return fmt.Sprintf("node-v%s-%s-%s", version, osName, arch)
}

func (bt nodeBuildTool) nodeDir() string {
	return filepath.Join(bt.installDir(), bt.packageString())
}

func (bt nodeBuildTool) installDir() string {
	return filepath.Join(bt.spec.sharedCacheDir, "nodejs")
}

func (bt nodeBuildTool) install(ctx context.Context) error {

	nodeDir := bt.nodeDir()
	installDir := bt.installDir()
	nodePkgString := bt.packageString()

	if _, err := os.Stat(nodeDir); err == nil {
		log.Infof("Node v%s located in %s!", bt.version, nodeDir)
	} else {
		log.Infof("Would install Node v%s into %s", bt.version, installDir)
		archiveFile := fmt.Sprintf("%s.tar.gz", nodePkgString)
		downloadUrl := fmt.Sprintf("%s/v%s/%s", nodeDistMirror, bt.version, archiveFile)
		log.Infof("Downloading from URL %s...", downloadUrl)
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
	}

	return nil
}

func (bt nodeBuildTool) setup(ctx context.Context) error {
	nodeDir := bt.nodeDir()
	cmdPath := filepath.Join(nodeDir, "bin")
	plumbing.PrependToPath(cmdPath)
	// TODO: Fix this to be the package cache?
	nodePath := bt.spec.packageDir
	log.Infof("Setting NODE_PATH to %s", nodePath)
	os.Setenv("NODE_PATH", nodePath)

	npmBinPath := filepath.Join(nodePath, "node_modules", ".bin")
	plumbing.PrependToPath(npmBinPath)

	return nil
}
