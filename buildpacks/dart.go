package buildpacks

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
)

//https://archive.apache.org/dist/dart/dart-3/3.3.3/binaries/apache-dart-3.3.3-bin.tar.gz
const dartDistMirrorTemplate = "https://storage.googleapis.com/dart-archive/channels/stable/release/{{.Version}}/sdk/dartsdk-{{.OS}}-{{.Arch}}-release.zip"

type DartBuildTool struct {
	version string
	spec    BuildToolSpec
}

func NewDartBuildTool(toolSpec BuildToolSpec) DartBuildTool {
	tool := DartBuildTool{
		version: toolSpec.Version,
		spec:    toolSpec,
	}

	return tool
}

func (bt DartBuildTool) DownloadURL(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	os := t.OS()
	opsys := "linux"
	architecture := t.Architecture()
	arch := "x64"
	extension := "zip"

	if architecture == runtime.I386 {
		arch = "x32"
	}

	if os == runtime.Darwin {
		opsys = "macos"
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

	url, err := TemplateToString(dartDistMirrorTemplate, data)

	return url, err
}

func (bt DartBuildTool) MajorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt DartBuildTool) Version() string {
	return bt.version
}

func (bt DartBuildTool) Setup(ctx context.Context, dartDir string) error {
	t := bt.spec.InstallTarget

	cmdPath := filepath.Join(dartDir, "bin")
	t.PrependToPath(ctx, cmdPath)

	return nil
}

func (bt DartBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "dart", bt.Version())
	dartDir := filepath.Join(installDir, "dart-sdk")

	if t.PathExists(ctx, dartDir) {
		log.Infof("Dart v%s located in %s!", bt.Version(), dartDir)
		return dartDir, nil
	}
	log.Infof("Will install Dart v%s into %s", bt.Version(), dartDir)
	downloadURL, err := bt.DownloadURL(ctx)
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}

	log.Infof("Downloading Dart from URL %s...", downloadURL)
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

	return dartDir, nil
}
