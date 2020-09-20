package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
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
	os := bt.spec.InstallTarget.OS()
	arch := bt.spec.InstallTarget.Architecture()
	osPart := "-linux"
	architecture := "-x86_64"
	ext := "zip"
	v := bt.Version()

	// awscli-exe-linux-x86_64-%s.zip for a specific version, e.g.: 2.0.49 (latest)
	// or awscli-exe-linux-x86_64.zip for the latest

	if os == runtime.Darwin {
		osPart = ""
		architecture = ""
		pkgName = "AWSCLIV2"
		ext = "pkg"
	} else if os == runtime.Windows {
		// TODO add support for Windows (for the whole CLI?)
		// dig http://stackoverflow.com/questions/8560166/ddg#8560308 for unattended MSI installs
		return ""
	}

	// TODO support arm64, armv6
	if arch != runtime.Amd64 {
		return ""
	}

	base := pkgName + osPart + architecture

	if v == "latest" {
		return base + "." + ext
	}

	return base + "-" + v + "." + ext
}

func (bt AWSCLIBuildTool) DownloadURL(ctx context.Context) (string, error) {
	// This version of the AWS CLI has a bundled-in Python, it will be Linux only for now
	const awsCLIBaseURL = "https://awscli.amazonaws.com/"
	archiveFile := bt.ArchiveFile()
	if archiveFile == "" {
		return "", fmt.Errorf("no support for arch/OS yet")
	}

	return awsCLIBaseURL + archiveFile, nil
}

func (bt AWSCLIBuildTool) Version() string {
	return bt.version
}

func (bt AWSCLIBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "awscli", bt.Version())

	if t.PathExists(ctx, installDir) {
		log.Infof("AWSCLI v%s located in %s!", bt.Version(), installDir)
		return installDir, nil
	}
	log.Infof("Will install AWSCLI v%s into %s", bt.Version(), installDir)

	downloadURL, err := bt.DownloadURL(ctx)
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}

	log.Infof("Downloading from URL %s ...", downloadURL)
	localFile, err := t.DownloadFile(ctx, downloadURL)
	if err != nil {
		log.Errorf("Unable to download: %v", err)
		return "", err
	}

	if bt.spec.InstallTarget.OS() == runtime.Darwin {
		return bt.darwinInstall(ctx, installDir)
	}

	err = t.Unarchive(ctx, localFile, installDir)
	if err != nil {
		log.Errorf("Unable to decompress: %v", err)
		return "", err
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
		err = t.Run(ctx, runtime.Process{
			Command:   cmd,
			Directory: installDir,
		})
		if err != nil {
			return "", err
		}
	}

	return installDir, nil
}

// darwinInstall is a branch from Install, that will do rootless AWSCLIv2 install on a Mac OS X
// NOTE: this needs the equivalent of Travis's Matrix support for our YAML, to be tested on the CI
func (bt AWSCLIBuildTool) darwinInstall(ctx context.Context, installDir string) (string, error) {
	log.Warnf("Installing AWSCLI on Darwin is only supported in Metal")

	customPkgXMLPath := filepath.Join(installDir, "choices.xml")

	customPkgXML := fmt.Sprintf(`
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <array>
    <dict>
      <key>choiceAttribute</key>
      <string>customLocation</string>
      <key>attributeSetting</key>
      <string>%s</string>
      <key>choiceIdentifier</key>
      <string>default</string>
    </dict>
  </array>
</plist>
`, customPkgXMLPath)

	t := bt.spec.InstallTarget
	err := t.WriteFileContents(ctx, customPkgXML, customPkgXMLPath)
	if err != nil {
		return "", err
	}

	err = t.Run(ctx, runtime.Process{
		Command: "installer -pkg " + bt.ArchiveFile() + " -target " + installDir + " -applyChoiceChangesXML choices.xml",
	})
	if err != nil {
		return "", err
	}

	return installDir, nil
}

func (bt AWSCLIBuildTool) Setup(ctx context.Context, installDir string) error {
	t := bt.spec.InstallTarget

	t.PrependToPath(ctx, filepath.Join(installDir, "bin"))

	cmd := "aws --version"
	log.Infof("Running: %v", cmd)
	err := t.Run(ctx, runtime.Process{
		Command:   cmd,
		Directory: bt.spec.PackageDir,
	})
	if err != nil {
		return err
	}

	return nil
}
