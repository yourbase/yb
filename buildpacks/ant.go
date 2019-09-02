package buildpacks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnewart/archiver"
	. "github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/types"
)

//http://apache.mirrors.lucidnetworks.net//ant/binaries/apache-ant-1.10.6-bin.tar.gz
var ANT_DIST_MIRROR = "http://apache.mirrors.lucidnetworks.net/ant/binaries/apache-ant-{{.Version}}-bin.zip"

type AntBuildTool struct {
	BuildTool
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

func (bt AntBuildTool) DownloadUrl() string {
	data := struct {
		Version string
	}{
		bt.Version(),
	}

	url, _ := TemplateToString(ANT_DIST_MIRROR, data)

	return url
}

func (bt AntBuildTool) MajorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt AntBuildTool) Version() string {
	return bt.version
}

func (bt AntBuildTool) AntDir() string {
	return filepath.Join(bt.InstallDir(), fmt.Sprintf("apache-ant-%s", bt.Version()))
}

func (bt AntBuildTool) InstallDir() string {
	return filepath.Join(bt.spec.SharedCacheDir, "ant")
}

func (bt AntBuildTool) Setup() error {
	antDir := bt.AntDir()

	cmdPath := filepath.Join(antDir, "bin")
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	log.Infof("Setting PATH to %s\n", newPath)
	os.Setenv("PATH", newPath)

	return nil
}

// TODO, generalize downloader
func (bt AntBuildTool) Install() error {
	antDir := bt.AntDir()
	installDir := bt.InstallDir()

	if _, err := os.Stat(antDir); err == nil {
		fmt.Printf("Ant v%s located in %s!\n", bt.Version(), antDir)
	} else {
		fmt.Printf("Will install Ant v%s into %s\n", bt.Version(), antDir)
		downloadUrl := bt.DownloadUrl()

		fmt.Printf("Downloading Ant from URL %s...\n", downloadUrl)
		localFile, err := DownloadFileWithCache(downloadUrl)
		if err != nil {
			fmt.Printf("Unable to download: %v\n", err)
			return err
		}
		err = archiver.Unarchive(localFile, installDir)
		if err != nil {
			fmt.Printf("Unable to decompress: %v\n", err)
			return err
		}

	}

	return nil
}
