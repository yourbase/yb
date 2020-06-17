package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/types"
)

var NODE_DIST_MIRROR = "https://nodejs.org/dist"

type NodeBuildTool struct {
	BuildTool
	version string
	spec    BuildToolSpec
}

func NewNodeBuildTool(toolSpec BuildToolSpec) NodeBuildTool {
	tool := NodeBuildTool{
		version: toolSpec.Version,
		spec:    toolSpec,
	}

	return tool
}

func (bt NodeBuildTool) Version() string {
	return bt.version
}
func (bt NodeBuildTool) PackageString() string {
	version := bt.Version()
	arch := Arch()

	if arch == "amd64" {
		arch = "x64"
	}

	osName := OS()

	return fmt.Sprintf("node-v%s-%s-%s", version, osName, arch)
}

func (bt NodeBuildTool) Install(ctx context.Context) (error, string) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "nodejs")
	nodePkgString := bt.PackageString()
	nodeDir := filepath.Join(installDir, nodePkgString)

	if t.PathExists(ctx, nodeDir) {
		log.Infof("Node v%s located in %s!", bt.Version(), nodeDir)
	} else {
		log.Infof("Would install Node v%s into %s", bt.Version(), installDir)
		archiveFile := fmt.Sprintf("%s.tar.gz", nodePkgString)
		downloadUrl := fmt.Sprintf("%s/v%s/%s", NODE_DIST_MIRROR, bt.Version(), archiveFile)
		log.Infof("Downloading from URL %s...", downloadUrl)
		localFile, err := t.DownloadFile(ctx, downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err, ""
		}

		err = t.Unarchive(ctx, localFile, installDir)
		if err != nil {
			log.Errorf("Unable to decompress: %v", err)
			return err, ""
		}
	}

	return nil, nodeDir
}

func (bt NodeBuildTool) Setup(ctx context.Context, nodeDir string) error {
	t := bt.spec.InstallTarget

	cmdPath := filepath.Join(nodeDir, "bin")
	t.PrependToPath(ctx, cmdPath)
	// TODO: Fix this to be the package cache?
	nodePath := bt.spec.PackageDir
	log.Infof("Setting NODE_PATH to %s", nodePath)
	t.SetEnv("NODE_PATH", nodePath)

	npmBinPath := filepath.Join(nodePath, "node_modules", ".bin")
	t.PrependToPath(ctx, npmBinPath)

	return nil
}
