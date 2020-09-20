package buildpacks

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/johnewart/archiver"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
)

type AWSCLIBuildTool struct {
	version string
	spec    BuildToolSpec
}

func NewAWSCLIBuildTool(toolSpec BuildToolSpec) AWSCLIBuildTool {
	tool := AWSCLIBuildTool{
		version: toolSpec.Version,
		spec:    toolSpec,
	}

	return tool
}

func (bt AWSCLIBuildTool) ArchiveFile() string {
	pkgName := "awscli-exe"
	os := OS()
	arch := Arch()
	osPart := "-linux"
	architecture := "-x86_64"
	ext := "zip"
	v := bt.Version()

	// awscli-exe-linux-x86_64-%s.zip for a specific version, e.g.: 2.0.49 (latest)
	// or awscli-exe-linux-x86_64.zip for the latest

	if os == "darwin" {
		osPart = ""
		architecture = ""
		pkgName = "AWSCLIV2"
		ext = "pkg"
	} else if os == "windows" {
		// TODO add support for Windows (for the whole CLI?)
		// dig http://stackoverflow.com/questions/8560166/ddg#8560308 for unattended MSI installs
		return ""
	}

	// TODO support arm64, armv6
	if arch != "amd64" {
		return ""
	}

	base := pkgName + osPart + architecture

	if v == "latest" {
		return base + "." + ext
	}

	return base + "-" + v + "." + ext
}

func (bt AWSCLIBuildTool) DownloadURL() string {
	// This version of the AWS CLI has a bundled-in Python, it will be Linux only for now
	const awsCLIBaseURL = "https://awscli.amazonaws.com/"
	archiveFile := bt.ArchiveFile()
	if archiveFile == "" {
		return ""
	}

	return awsCLIBaseURL + archiveFile
}

func (bt AWSCLIBuildTool) InstallDir() string {
	return filepath.Join(bt.spec.SharedCacheDir, "awscli", bt.Version())
}

func (bt AWSCLIBuildTool) Version() string {
	return bt.version
}

func (bt AWSCLIBuildTool) Install() error {
	installDir := bt.InstallDir()

	if _, err := os.Stat(installDir); err == nil {
		log.Infof("AWSCLI v%s located in %s!", bt.Version(), installDir)
		return nil
	}

	if OS() == "darwin" {
		return fmt.Errorf("Work in progress, please try v0.2.x")
	}

	log.Infof("Will install AWSCLI v%s into %s", bt.Version(), installDir)
	downloadURL := bt.DownloadURL()
	log.Infof("Downloading from URL %s ...", downloadURL)
	localFile, err := plumbing.DownloadFileWithCache(downloadURL)
	if err != nil {
		log.Errorf("Unable to download: %v", err)
		return err
	}

	err = archiver.Unarchive(localFile, installDir)
	if err != nil {
		return err
	}

	var updateString string
	if bt.Version() == "latest" {
		updateString = "--update "
	}

	// Linux installation
	// NOTE Needs to be run as root (sudo)
	for _, cmd := range []string{
		"mkdir -p bin",
		"./aws/install " + updateString + "--install-dir " + installDir + " --bin-dir ./bin",
	} {
		err = plumbing.ExecToStdout(cmd, installDir)
		if err != nil {
			return err
		}
	}

	return nil
}

func (bt AWSCLIBuildTool) Setup() error {

	plumbing.PrependToPath(filepath.Join(bt.InstallDir(), "bin"))

	cmd := "aws --version"
	log.Infof("Running: %v", cmd)
	err := plumbing.ExecToStdout(cmd, bt.InstallDir())
	if err != nil {
		return err
	}

	return nil
}
