package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/yourbase/yb/plumbing/log"
)

const nodeDistMirrorTemplate = "https://nodejs.org/dist"

type NodeBuildTool struct {
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

func (bt NodeBuildTool) ArchiveFile() string {
	return fmt.Sprintf("%s.tar.gz", bt.PackageString())
}

func (bt NodeBuildTool) DownloadURL(ctx context.Context) (string, error) {
	return fmt.Sprintf("%s/v%s/%s",
		nodeDistMirrorTemplate,
		bt.Version(),
		bt.ArchiveFile()), nil
}

func (bt NodeBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "nodejs")
	nodeDir := filepath.Join(installDir, bt.PackageString())

	if t.PathExists(ctx, nodeDir) {
		log.Infof("Node v%s located in %s!", bt.Version(), nodeDir)
		return nodeDir, nil
	}
	log.Infof("Would install Node v%s into %s", bt.Version(), installDir)
	downloadURL, err := bt.DownloadURL(ctx)
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}

	log.Infof("Downloading from URL %s...", downloadURL)
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

	return nodeDir, nil
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
