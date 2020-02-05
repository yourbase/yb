package buildpacks

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/yourbase/yb/plumbing"
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

func (bt RustBuildTool) RustDir() string {
	return filepath.Join(bt.InstallDir(), fmt.Sprintf("rust-%s", bt.Version()))
}

func (bt RustBuildTool) InstallDir() string {
	return filepath.Join(bt.spec.SharedCacheDir, "rust")
}

func (bt RustBuildTool) Setup() error {
	rustDir := bt.RustDir()
	cmdPath := fmt.Sprintf("%s/bin", rustDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	log.Infof("Setting PATH to %s", newPath)
	runtime.SetEnv("PATH", newPath)

	runtime.SetEnv("CARGO_HOME", rustDir)
	runtime.SetEnv("RUSTUP_HOME", rustDir)

	return nil
}

func (bt RustBuildTool) Install() error {

	arch := "x86_64"
	operatingSystem := "unknown-linux-gnu"

	rustDir := bt.RustDir()
	installDir := bt.InstallDir()
	MkdirAsNeeded(installDir)

	if _, err := os.Stat(rustDir); err == nil {
		log.Infof("Rust v%s located in %s!", bt.Version(), rustDir)
	} else {
		log.Infof("Will install Rust v%s into %s", bt.Version(), rustDir)
		extension := ""
		installerFile := fmt.Sprintf("rustup-init%s", extension)
		downloadUrl := fmt.Sprintf("%s/%s-%s/%s", RUST_DIST_MIRROR, arch, operatingSystem, installerFile)

		downloadDir := bt.spec.PackageCacheDir
		localFile := filepath.Join(downloadDir, installerFile)
		log.Infof("Downloading from URL %s to local file %s", downloadUrl, localFile)
		localFile, err := bt.spec.InstallTarget.DownloadFile(downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err
		}

		os.Chmod(localFile, 0700)
		runtime.SetEnv("CARGO_HOME", rustDir)
		runtime.SetEnv("RUSTUP_HOME", rustDir)

		installCmd := fmt.Sprintf("%s -y", localFile)
		runtime.ExecToStdout(installCmd, downloadDir)
	}

	return nil

}
