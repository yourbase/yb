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

// https://github.com/google/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip
const ProtocDistMirror = "https://github.com/google/protobuf/releases/download/v{{.Version}}/protoc-{{.Version}}-{{.OS}}-x86_64.{{.Extension}}"

type ProtocBuildTool struct {
	BuildTool
	version string
	spec    BuildToolSpec
}

func NewProtocBuildTool(spec BuildToolSpec) ProtocBuildTool {

	tool := ProtocBuildTool{
		version: spec.Version,
		spec:    spec,
	}

	return tool
}

func (bt ProtocBuildTool) DownloadUrl() string {
	opsys := OS()
	arch := Arch()
	extension := "zip"

	if arch == "amd64" {
		arch = "x64"
	}

	if opsys == "darwin" {
		opsys = "osx"
	}

	version := bt.Version()

	data := struct {
		OS        string
		Arch      string
		Version   string
		Extension string
	}{
		opsys,
		arch,
		version,
		extension,
	}

	url, _ := TemplateToString(ProtocDistMirror, data)

	return url
}

func (bt ProtocBuildTool) MajorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt ProtocBuildTool) Version() string {
	return bt.version
}

func (bt ProtocBuildTool) InstallDir() string {
	return filepath.Join(bt.spec.PackageCacheDir, "protoc", fmt.Sprintf("protoc-%s", bt.Version()))
}

func (bt ProtocBuildTool) ProtocDir() string {
	return filepath.Join(bt.InstallDir())
}

func (bt ProtocBuildTool) Setup() error {
	protocDir := bt.ProtocDir()

	cmdPath := filepath.Join(protocDir, "bin")
	PrependToPath(cmdPath)
	return nil
}

// TODO, generalize downloader
func (bt ProtocBuildTool) Install() error {
	protocDir := bt.ProtocDir()
	installDir := bt.InstallDir()

	if _, err := os.Stat(protocDir); err == nil {
		log.Infof("Protoc v%s located in %s!", bt.Version(), protocDir)
		return nil
	}
	log.Infof("Will install Protoc v%s into %s", bt.Version(), protocDir)
	downloadUrl := bt.DownloadUrl()

	log.Infof("Downloading Protoc from URL %s...", downloadUrl)
	localFile, err := DownloadFileWithCache(downloadUrl)
	if err != nil {
		log.Errorf("Unable to download: %v", err)
		return err
	}
	err = archiver.Unarchive(localFile, installDir)
	if err != nil {
		log.Errorf("Unable to decompress: %v", err)
		return err
	}

	//RemoveWritePermissionRecursively(installDir)

	return nil
}
