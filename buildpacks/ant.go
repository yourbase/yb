package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yourbase/yb/plumbing/log"
)

//http://apache.mirrors.lucidnetworks.net//ant/binaries/apache-ant-1.10.6-bin.tar.gz
const antDistMirrorTemplate = "http://apache.mirrors.lucidnetworks.net/ant/binaries/apache-ant-{{.Version}}-bin.zip"

type AntBuildTool struct {
	version string
	spec    BuildToolSpec
}

func NewAntBuildTool(toolSpec BuildToolSpec) AntBuildTool {

	tool := AntBuildTool{
		version: toolSpec.Version,
		spec:    toolSpec,
	}

	return tool
}

func (bt AntBuildTool) ArchiveFile() string {
	return fmt.Sprintf("apache-ant-%s-bin.zip", bt.Version())
}

func (bt AntBuildTool) DownloadURL(ctx context.Context) (string, error) {
	data := struct {
		Version string
	}{
		bt.Version(),
	}

	url, err := TemplateToString(antDistMirrorTemplate, data)

	return url, err
}

func (bt AntBuildTool) MajorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt AntBuildTool) Version() string {
	return bt.version
}

func (bt AntBuildTool) Setup(ctx context.Context, antDir string) error {
	t := bt.spec.InstallTarget

	t.PrependToPath(ctx, filepath.Join(antDir, "bin"))

	return nil
}

// TODO, generalize downloader
func (bt AntBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "ant")
	antDir := filepath.Join(installDir, "apache-ant-"+bt.Version())

	if t.PathExists(ctx, antDir) {
		log.Infof("Ant v%s located in %s!", bt.Version(), antDir)
		return antDir, nil
	}

	log.Infof("Will install Ant v%s into %s", bt.Version(), antDir)
	downloadURL, err := bt.DownloadURL(ctx)
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}

	log.Infof("Downloading Ant from URL %s...", downloadURL)
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

	return antDir, nil
}
