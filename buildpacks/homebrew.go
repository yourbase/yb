package buildpacks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/matishsiao/goInfo"
	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
	"gopkg.in/src-d/go-git.v4"
)

type HomebrewBuildTool struct {
	BuildTool
	version string
	spec    BuildToolSpec
	pkgName string
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

	fmt.Printf("Installing package %s\n", bt.pkgName)
	if bt.pkgName != "" {
		err = bt.InstallPackage()
		if err != nil {
			return err
		}
	}

	return nil
}

func (bt HomebrewBuildTool) InstallPackage() error {
	bt.Setup()

	brewDir := bt.HomebrewDir()

	updateCmd := "brew update"
	err := ExecToStdout(updateCmd, brewDir)
	if err != nil {
		return fmt.Errorf("Couldn't update brew: %v", err)
	}

	pkgVersion := ""
	if bt.version != "" {
		pkgVersion = fmt.Sprintf("@%s", bt.version)
	}

	fmt.Printf("Going to install %s%s from Homebrew...\n", bt.pkgName, pkgVersion)
	installCmd := fmt.Sprintf("brew install %s%s", bt.pkgName, pkgVersion)
	err = ExecToStdout(installCmd, brewDir)

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
		fmt.Printf("brew installed in %s\n", brewDir)
	} else {
		fmt.Printf("Installing brew\n")

		_, err := git.PlainClone(brewDir, false, &git.CloneOptions{
			URL:      brewGitUrl,
			Progress: os.Stdout,
		})

		if err != nil {
			fmt.Printf("Unable to clone brew!\n")
			return fmt.Errorf("Couldn't clone brew: %v\n", err)
		}
	}
	fmt.Printf("Updating brew\n")
	updateCmd := "brew update"
	ExecToStdout(updateCmd, brewDir)

	return nil
}

func (bt HomebrewBuildTool) InstallLinux() error {
	installDir := bt.InstallDir()
	brewDir := bt.HomebrewDir()

	MkdirAsNeeded(installDir)

	brewGitUrl := "https://github.com/Homebrew/brew.git"

	bt.InstallPlatformDependencies()

	if _, err := os.Stat(brewDir); err == nil {
		fmt.Printf("brew installed in %s\n", brewDir)
	} else {
		fmt.Printf("Installing brew\n")

		_, err := git.PlainClone(brewDir, false, &git.CloneOptions{
			URL:      brewGitUrl,
			Progress: os.Stdout,
		})

		if err != nil {
			fmt.Printf("Unable to clone brew!\n")
			return fmt.Errorf("Couldn't clone brew: %v\n", err)
		}
	}
	fmt.Printf("Updating brew\n")
	updateCmd := "brew update"
	ExecToStdout(updateCmd, brewDir)
	return nil
}

func (bt HomebrewBuildTool) Setup() error {
	brewDir := bt.HomebrewDir()
	brewBinDir := filepath.Join(brewDir, "bin")

	PrependToPath(brewBinDir)

	return nil
}

func (bt HomebrewBuildTool) InstallPlatformDependencies() error {
	// Currently a no-op
	return nil
}
