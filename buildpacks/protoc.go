package buildpacks

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
)

// https://github.com/google/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip
const protocDistMirror = "https://github.com/google/protobuf/releases/download/v{{.Version}}/protoc-{{.Version}}-{{.OS}}-{{.Arch}}.{{.Extension}}"

type ProtocBuildTool struct {
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

func (bt ProtocBuildTool) DownloadURL(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget
	os := t.OS()
	arch := t.Architecture()

	opsys := "linux"
	archLabel := "x86_64"
	extension := "zip"

	if arch == runtime.I386 {
		archLabel = "x86_32"
	}

	if os == runtime.Darwin {
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
		archLabel,
		version,
		extension,
	}

	url, err := TemplateToString(protocDistMirror, data)
	return url, err
}

func (bt ProtocBuildTool) MajorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt ProtocBuildTool) Version() string {
	return bt.version
}

func (bt ProtocBuildTool) Setup(ctx context.Context, protocDir string) error {
	t := bt.spec.InstallTarget

	cmdPath := filepath.Join(protocDir, "bin")
	t.PrependToPath(ctx, cmdPath)
	return nil
}

func (bt ProtocBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "protoc")
	protocDir := filepath.Join(installDir, "protoc-"+bt.Version())

	if t.PathExists(ctx, protocDir) {
		log.Infof("Protoc v%s located in %s!", bt.Version(), protocDir)
		return protocDir, nil
	}
	log.Infof("Will install Protoc v%s into %s", bt.Version(), protocDir)
	downloadURL, err := bt.DownloadURL(ctx)
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}

	log.Infof("Downloading Protoc from URL %s...", downloadURL)
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

	return protocDir, nil
}
