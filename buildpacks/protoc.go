package buildpacks

import (
	"context"
	"path/filepath"
	"strings"

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

func (bt ProtocBuildTool) Setup(ctx context.Context, protocDir string) error {
	t := bt.spec.InstallTarget

	cmdPath := filepath.Join(protocDir, "bin")
	t.PrependToPath(ctx, cmdPath)
	return nil
}

func (bt ProtocBuildTool) Install(ctx context.Context) (error, string) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "protoc")
	protocDir := filepath.Join(installDir, "protoc-"+bt.Version())

	if t.PathExists(ctx, protocDir) {
		log.Infof("Protoc v%s located in %s!", bt.Version(), protocDir)
		return nil, ""
	}
	log.Infof("Will install Protoc v%s into %s", bt.Version(), protocDir)
	downloadUrl := bt.DownloadUrl()

	log.Infof("Downloading Protoc from URL %s...", downloadUrl)
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

	return nil, protocDir
}
