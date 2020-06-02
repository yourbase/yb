package buildpacks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/matishsiao/goInfo"
	. "github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
	. "github.com/yourbase/yb/types"
	"gopkg.in/src-d/go-git.v4"
)

type HomebrewBuildTool struct {
	BuildTool
	version   string
	spec      BuildToolSpec
	pkgName   string
	pkgPrefix string
}

func NewHomebrewBuildTool(toolSpec BuildToolSpec) HomebrewBuildTool {

	pkgName := ""
	version := toolSpec.Version

	parts := strings.Split(version, "@")
	if len(parts) == 2 {
		// Package
		pkgName = parts[0]
		version = parts[1]
	} else {
		// Homebrew itself
		version = toolSpec.Version
	}

	tool := HomebrewBuildTool{
		version: version,
		spec:    toolSpec,
		pkgName: pkgName,
	}

	return tool
}

func (bt HomebrewBuildTool) Version() string {
	return bt.version
}

func (bt HomebrewBuildTool) IsPackage() bool {
	return bt.pkgName != ""
}

func (bt HomebrewBuildTool) PackagePrefix(packageString string) (string, error) {
	if bt.pkgPrefix != "" {
		return bt.pkgPrefix, nil
	}

	output, err := exec.Command("brew", "--prefix", packageString).Output()
	if err != nil {
		return "", fmt.Errorf("Couldn't get prefix for package %s: %v", bt.pkgName, err)
	}

	prefixPath := string(output)
	prefixPath = strings.TrimSuffix(prefixPath, "\n")
	bt.pkgPrefix = prefixPath
	log.Debugf("Prefix for homebrew package %s is %s", bt.pkgName, prefixPath)

	return prefixPath, nil
}

func (bt HomebrewBuildTool) PackageInstalled() bool {
	prefix, err := bt.PackagePrefix(bt.PackageVersionString())
	t := bt.spec.InstallTarget
	if err != nil {
		return false
	}

	if prefix != "" {
		return t.PathExists(prefix)
	}

	return false
}

func (bt HomebrewBuildTool) PackageVersionString() string {
	pkgVersion := ""
	if bt.version != "" {
		pkgVersion = fmt.Sprintf("@%s", bt.version)
	}

	return fmt.Sprintf("%s%s", bt.pkgName, pkgVersion)
}

// Normally we want to put this in the tools dir; for now we put it in the build dir because I'm not
// sure how to handle installation of multiple versions of things via Brew so this will allow project-specific
// versioning
func (bt HomebrewBuildTool) HomebrewDir() string {
	return filepath.Join(bt.InstallDir(), "brew")
}

func (bt HomebrewBuildTool) InstallDir() string {
	return filepath.Join(bt.spec.PackageCacheDir, "homebrew")
}

func (bt HomebrewBuildTool) Install() error {
	gi := goInfo.GetInfo()

	var err error
	switch gi.GoOS {
	case "darwin":
		err = bt.InstallDarwin()
	case "linux":
		err = bt.InstallLinux()
	default:
		err = fmt.Errorf("Unsupported platform: %s", gi.GoOS)
	}

	if err != nil {
		return fmt.Errorf("Unable to install Homebrew: %v", err)
	}

	if bt.IsPackage() {
		if bt.PackageInstalled() {
			log.Infof("Package %s already installed", bt.PackageVersionString())
		} else {
			log.Infof("Installing package %s", bt.PackageVersionString())
			err = bt.InstallPackage()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (bt HomebrewBuildTool) InstallPackage() error {
	bt.Setup()

	brewDir := bt.HomebrewDir()

	updateCmd := "brew update"
	err := runtime.ExecToStdout(updateCmd, brewDir)
	if err != nil {
		return fmt.Errorf("Couldn't update brew: %v", err)
	}

	pkgVersion := ""
	if bt.version != "" {
		pkgVersion = fmt.Sprintf("@%s", bt.version)
	}

	log.Infof("Going to install %s%s from Homebrew...", bt.pkgName, pkgVersion)
	installCmd := fmt.Sprintf("brew install %s%s", bt.pkgName, pkgVersion)
	err = runtime.ExecToStdout(installCmd, brewDir)

	if err != nil {
		return fmt.Errorf("Couldn't intall %s@%s from  Homebrew: %v", bt.pkgName, bt.version, err)
	}

	return nil
}

func (bt HomebrewBuildTool) InstallDarwin() error {
	installDir := bt.InstallDir()
	brewDir := bt.HomebrewDir()

	MkdirAsNeeded(installDir)

	brewGitUrl := "https://github.com/Homebrew/brew.git"

	if _, err := os.Stat(brewDir); err == nil {
		log.Infof("brew installed in %s", brewDir)
	} else {
		log.Infof("Installing brew")

		_, err := git.PlainClone(brewDir, false, &git.CloneOptions{
			URL:      brewGitUrl,
			Progress: os.Stdout,
		})

		if err != nil {
			log.Errorf("Unable to clone brew!")
			return fmt.Errorf("Couldn't clone brew: %v", err)
		}
	}
	log.Infof("Updating brew")
	updateCmd := "brew update"
	runtime.ExecToStdout(updateCmd, brewDir)

	return nil
}

func (bt HomebrewBuildTool) InstallLinux() error {
	installDir := bt.InstallDir()
	brewDir := bt.HomebrewDir()

	MkdirAsNeeded(installDir)

	brewGitUrl := "https://github.com/Homebrew/brew.git"

	bt.InstallPlatformDependencies()

	if _, err := os.Stat(brewDir); err == nil {
		log.Infof("brew installed in %s", brewDir)
	} else {
		log.Infof("Installing brew")

		_, err := git.PlainClone(brewDir, false, &git.CloneOptions{
			URL:      brewGitUrl,
			Progress: os.Stdout,
		})

		if err != nil {
			log.Errorf("Unable to clone brew!")
			return fmt.Errorf("Couldn't clone brew: %v", err)
		}

		log.Infof("Updating brew")
		updateCmd := "brew update"
		runtime.ExecToStdout(updateCmd, brewDir)
	}
	return nil
}

func (bt HomebrewBuildTool) Setup() error {
	t := bt.spec.InstallTarget
	if bt.IsPackage() {
		prefixPath, err := bt.PackagePrefix(bt.PackageVersionString())
		if err != nil {
			return fmt.Errorf("Unable to determine prefix for package %s: %v", bt.PackageVersionString(), err)
		}
		binDir := filepath.Join(prefixPath, "bin")
		sbinDir := filepath.Join(prefixPath, "sbin")

		t.PrependToPath(binDir)
		t.PrependToPath(sbinDir)
	} else {
		brewDir := bt.HomebrewDir()
		brewBinDir := filepath.Join(brewDir, "bin")
		t.PrependToPath(brewBinDir)
		brewLibDir := filepath.Join(brewDir, "lib")
		runtime.SetEnv("LD_LIBRARY_PATH", brewLibDir)
	}
	return nil
}

func (bt HomebrewBuildTool) InstallPlatformDependencies() error {
	// Currently a no-op
	return nil
}
