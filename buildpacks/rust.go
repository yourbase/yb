package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
)

const rustDistMirrorTemplate = "https://static.rust-lang.org/rustup/dist"

type RustBuildTool struct {
	version string
	spec    BuildToolSpec
}

func NewRustBuildTool(toolSpec BuildToolSpec) RustBuildTool {
	tool := RustBuildTool{
		version: toolSpec.Version,
		spec:    toolSpec,
	}

	return tool
}

func (bt RustBuildTool) Version() string {
	return bt.version
}

func (bt RustBuildTool) Setup(ctx context.Context, rustDir string) error {
	t := bt.spec.InstallTarget

	t.PrependToPath(ctx, filepath.Join(rustDir, "bin"))

	t.SetEnv("CARGO_HOME", filepath.Join(t.ToolOutputSharedDir(ctx), "rust", bt.Version()))
	t.SetEnv("RUSTUP_HOME", rustDir)

	return nil
}

func (bt RustBuildTool) installerFile() string {
	extension := ""
	return fmt.Sprintf("rustup-init%s", extension)
}

func (bt RustBuildTool) DownloadURL(ctx context.Context) (string, error) {
	arch := "x86_64"
	operatingSystem := "unknown-linux-gnu"

	return fmt.Sprintf("%s/%s-%s/%s", rustDistMirrorTemplate, arch, operatingSystem, bt.installerFile()), nil
}

func (bt RustBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "rust")
	rustDir := filepath.Join(installDir, "rust-"+bt.Version())
	t.MkdirAsNeeded(ctx, installDir)

	if t.PathExists(ctx, rustDir) {
		log.Infof("Rust v%s located in %s!", bt.Version(), rustDir)
		return rustDir, nil
	}
	log.Infof("Will install Rust v%s into %s", bt.Version(), rustDir)
	downloadURL, err := bt.DownloadURL(ctx)
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}

	downloadDir := t.ToolsDir(ctx)
	localFile := filepath.Join(downloadDir, bt.installerFile())
	log.Infof("Downloading from URL %s to local file %s", downloadURL, localFile)
	localFile, err = t.DownloadFile(ctx, downloadURL)
	if err != nil {
		log.Errorf("Unable to download: %v", err)
		return "", err
	}

	t.SetEnv("CARGO_HOME", rustDir)
	t.SetEnv("RUSTUP_HOME", rustDir)

	_, newName := filepath.Split(localFile)
	renamedFilePath := filepath.Join(downloadDir, newName)

	for _, cmd := range []string{
		"chmod +x " + localFile,
		"mv " + localFile + " " + renamedFilePath,
		"./" + newName + " -y",
	} {
		log.Infof("Running %v", cmd)
		p := runtime.Process{
			Command:   cmd,
			Directory: downloadDir,
		}
		err = t.Run(ctx, p)
		if err != nil {
			return "", err
		}
	}

	return rustDir, nil

}
