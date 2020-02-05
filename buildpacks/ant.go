package buildpacks

import (
	"fmt"
	"github.com/yourbase/yb/runtime"
	"os"
	"path/filepath"
	"strings"

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
	log.Infof("Setting PATH to %s", newPath)
	runtime.SetEnv("PATH", newPath)

	return nil
}

// TODO, generalize downloader
func (bt AntBuildTool) Install() error {
	antDir := bt.AntDir()
	installDir := bt.InstallDir()

	if _, err := os.Stat(antDir); err == nil {
		log.Infof("Ant v%s located in %s!", bt.Version(), antDir)
	} else {
		log.Infof("Will install Ant v%s into %s", bt.Version(), antDir)
		downloadUrl := bt.DownloadUrl()

		log.Infof("Downloading Ant from URL %s...", downloadUrl)
		localFile, err := bt.spec.InstallTarget.DownloadFile(downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err
		}
		err = bt.spec.InstallTarget.Unarchive(localFile, installDir)
		if err != nil {
			log.Errorf("Unable to decompress: %v", err)
			return err
		}

	}

	return nil
}
