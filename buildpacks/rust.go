package buildpacks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
)

const rustDistMirror = "https://static.rust-lang.org/rustup/dist"

type rustBuildTool struct {
	version string
	spec    buildToolSpec
}

func newRustBuildTool(toolSpec buildToolSpec) rustBuildTool {
	tool := rustBuildTool{
		version: toolSpec.version,
		spec:    toolSpec,
	}

	return tool
}

func (bt rustBuildTool) rustDir() string {
	return filepath.Join(bt.installDir(), fmt.Sprintf("rust-%s", bt.version))
}

func (bt rustBuildTool) installDir() string {
	return filepath.Join(bt.spec.sharedCacheDir, "rust")
}

func (bt rustBuildTool) setup(ctx context.Context) error {
	rustDir := bt.rustDir()
	cmdPath := fmt.Sprintf("%s/bin", rustDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	log.Infof("Setting PATH to %s", newPath)
	os.Setenv("PATH", newPath)

	os.Setenv("CARGO_HOME", rustDir)
	os.Setenv("RUSTUP_HOME", rustDir)

	return nil
}

func (bt rustBuildTool) install(ctx context.Context) error {

	arch := "x86_64"
	operatingSystem := "unknown-linux-gnu"

	rustDir := bt.rustDir()
	installDir := bt.installDir()
	if err := os.MkdirAll(installDir, 0777); err != nil {
		return fmt.Errorf("install Rust: %w", err)
	}

	if _, err := os.Stat(rustDir); err == nil {
		log.Infof("Rust v%s located in %s!", bt.version, rustDir)
	} else {
		log.Infof("Will install Rust v%s into %s", bt.version, rustDir)
		extension := ""
		installerFile := fmt.Sprintf("rustup-init%s", extension)
		downloadUrl := fmt.Sprintf("%s/%s-%s/%s", rustDistMirror, arch, operatingSystem, installerFile)

		downloadDir := bt.spec.packageCacheDir
		localFile := filepath.Join(downloadDir, installerFile)
		log.Infof("Downloading from URL %s to local file %s", downloadUrl, localFile)
		err := plumbing.DownloadFile(localFile, downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err
		}

		os.Chmod(localFile, 0700)
		os.Setenv("CARGO_HOME", rustDir)
		os.Setenv("RUSTUP_HOME", rustDir)

		installCmd := fmt.Sprintf("%s -y", localFile)
		plumbing.ExecToStdout(installCmd, downloadDir)
	}

	return nil

}
