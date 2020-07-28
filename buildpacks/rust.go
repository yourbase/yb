package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
	. "github.com/yourbase/yb/types"
)

var RUST_DIST_MIRROR = "https://static.rust-lang.org/rustup/dist"

type RustBuildTool struct {
	BuildTool
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

	t.SetEnv("CARGO_HOME", rustDir)
	t.SetEnv("RUSTUP_HOME", rustDir)

	return nil
}

func (bt RustBuildTool) Install(ctx context.Context) (error, string) {
	t := bt.spec.InstallTarget

	arch := "x86_64"
	operatingSystem := "unknown-linux-gnu"

	installDir := filepath.Join(t.ToolsDir(ctx), "rust")
	rustDir := filepath.Join(installDir, "rust-"+bt.Version())
	t.MkdirAsNeeded(ctx, installDir)

	if t.PathExists(ctx, rustDir) {
		log.Infof("Rust v%s located in %s!", bt.Version(), rustDir)
	} else {
		log.Infof("Will install Rust v%s into %s", bt.Version(), rustDir)
		extension := ""
		installerFile := fmt.Sprintf("rustup-init%s", extension)
		downloadUrl := fmt.Sprintf("%s/%s-%s/%s", RUST_DIST_MIRROR, arch, operatingSystem, installerFile)

		downloadDir := t.ToolsDir(ctx)
		localFile := filepath.Join(downloadDir, installerFile)
		log.Infof("Downloading from URL %s to local file %s", downloadUrl, localFile)
		localFile, err := t.DownloadFile(ctx, downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err, ""
		}

		t.SetEnv("CARGO_HOME", rustDir)
		t.SetEnv("RUSTUP_HOME", rustDir)

		for _, cmd := range []string{
			"chmod +x " + localFile,
			localFile + " -y",
		} {
			p := runtime.Process{
				Command:   cmd,
				Directory: downloadDir,
			}
			err = t.Run(ctx, p)
			if err != nil {
				return err, ""
			}
		}
	}

	return nil, rustDir

}
