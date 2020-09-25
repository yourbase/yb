package buildpacks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnewart/archiver"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
)

// https://github.com/google/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip
const protocDistMirror = "https://github.com/google/protobuf/releases/download/v{{.Version}}/protoc-{{.Version}}-{{.OS}}-x86_64.{{.Extension}}"

type protocBuildTool struct {
	version string
	spec    buildToolSpec
}

func newProtocBuildTool(spec buildToolSpec) protocBuildTool {

	tool := protocBuildTool{
		version: spec.version,
		spec:    spec,
	}

	return tool
}

func (bt protocBuildTool) downloadURL() string {
	opsys := OS()
	arch := Arch()
	extension := "zip"

	if arch == "amd64" {
		arch = "x64"
	}

	if opsys == "darwin" {
		opsys = "osx"
	}

	version := bt.version

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

	url, _ := plumbing.TemplateToString(protocDistMirror, data)

	return url
}

func (bt protocBuildTool) majorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt protocBuildTool) installDir() string {
	return filepath.Join(bt.spec.packageCacheDir, "protoc", fmt.Sprintf("protoc-%s", bt.version))
}

func (bt protocBuildTool) protocDir() string {
	return filepath.Join(bt.installDir())
}

func (bt protocBuildTool) setup(ctx context.Context) error {
	protocDir := bt.protocDir()

	cmdPath := filepath.Join(protocDir, "bin")
	plumbing.PrependToPath(cmdPath)
	return nil
}

// TODO, generalize downloader
func (bt protocBuildTool) install(ctx context.Context) error {
	protocDir := bt.protocDir()
	installDir := bt.installDir()

	if _, err := os.Stat(protocDir); err == nil {
		log.Infof("Protoc v%s located in %s!", bt.version, protocDir)
		return nil
	}
	log.Infof("Will install Protoc v%s into %s", bt.version, protocDir)
	downloadUrl := bt.downloadURL()

	log.Infof("Downloading Protoc from URL %s...", downloadUrl)
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

	return nil
}
