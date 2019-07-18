package buildpacks

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/matishsiao/goInfo"
	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
	"gopkg.in/src-d/go-git.v4"
)

type HomebrewBuildTool struct {
	BuildTool
	version string
	spec    BuildToolSpec
}

func NewHomebrewBuildTool(toolSpec BuildToolSpec) HomebrewBuildTool {
	tool := HomebrewBuildTool{
		version: toolSpec.Version,
		spec:    toolSpec,
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

	switch gi.GoOS {
	case "darwin":
		return bt.InstallDarwin()
	case "linux":
		return bt.InstallLinux()
	default:
		fmt.Printf("Unsupported platform: %s\n", gi.GoOS)
		return nil
	}
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
