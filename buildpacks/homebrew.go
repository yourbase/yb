package buildpacks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yourbase/yb/plumbing"
	"gopkg.in/src-d/go-git.v4"
	"zombiezen.com/go/log"
)

type homebrewBuildTool struct {
	version   string
	spec      buildToolSpec
	pkgName   string
	pkgPrefix string
}

func newHomebrewBuildTool(toolSpec buildToolSpec) homebrewBuildTool {

	pkgName := ""
	version := toolSpec.version

	parts := strings.Split(version, "@")
	if len(parts) == 2 {
		// Package
		pkgName = parts[0]
		version = parts[1]
	} else {
		// Homebrew itself
		version = toolSpec.version
	}

	tool := homebrewBuildTool{
		version: version,
		spec:    toolSpec,
		pkgName: pkgName,
	}

	return tool
}

func (bt homebrewBuildTool) isPackage() bool {
	return bt.pkgName != ""
}

func (bt homebrewBuildTool) packagePrefix(ctx context.Context, packageString string) (string, error) {
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
	log.Debugf(ctx, "Prefix for homebrew package %s is %s", bt.pkgName, prefixPath)

	return prefixPath, nil
}

func (bt homebrewBuildTool) packageInstalled(ctx context.Context) bool {
	prefix, err := bt.packagePrefix(ctx, bt.packageVersionString())
	if err != nil {
		return false
	}

	if prefix != "" {
		return plumbing.PathExists(prefix)
	}

	return false
}

func (bt homebrewBuildTool) packageVersionString() string {
	pkgVersion := ""
	if bt.version != "" {
		pkgVersion = fmt.Sprintf("@%s", bt.version)
	}

	return fmt.Sprintf("%s%s", bt.pkgName, pkgVersion)
}

// Normally we want to put this in the tools dir; for now we put it in the build dir because I'm not
// sure how to handle installation of multiple versions of things via Brew so this will allow project-specific
// versioning
func (bt homebrewBuildTool) homebrewDir() string {
	return filepath.Join(bt.installDir(), "brew")
}

func (bt homebrewBuildTool) installDir() string {
	return filepath.Join(bt.spec.cacheDir, "homebrew")
}

func (bt homebrewBuildTool) install(ctx context.Context) error {
	if err := bt.installBrew(ctx); err != nil {
		return err
	}
	if !bt.isPackage() {
		return nil
	}
	if bt.packageInstalled(ctx) {
		log.Infof(ctx, "Package %s already installed", bt.packageVersionString())
		return nil
	}
	log.Infof(ctx, "Installing package %s", bt.packageVersionString())
	err := bt.installPackage(ctx)
	if err != nil {
		return fmt.Errorf("install homebrew: %w", err)
	}
	return nil
}

func (bt homebrewBuildTool) installBrew(ctx context.Context) error {
	// If directory already exists, skip the install.
	brewDir := bt.homebrewDir()
	if _, err := os.Stat(brewDir); err == nil {
		log.Infof(ctx, "brew installed in %s", brewDir)
		return nil
	}

	log.Infof(ctx, "Installing brew")
	installDir := bt.installDir()
	if err := os.MkdirAll(installDir, 0777); err != nil {
		return fmt.Errorf("install homebrew: %w", err)
	}
	_, err := git.PlainClone(brewDir, false, &git.CloneOptions{
		URL:      "https://github.com/Homebrew/brew.git",
		Progress: os.Stdout,
	})
	if err != nil {
		return fmt.Errorf("install homebrew: %w", err)
	}

	log.Infof(ctx, "Updating brew")
	if err := plumbing.ExecToStdout("brew update", brewDir); err != nil {
		return fmt.Errorf("install homebrew: brew update: %w", err)
	}
	return nil
}

func (bt homebrewBuildTool) installPackage(ctx context.Context) error {
	bt.setup(ctx)

	brewDir := bt.homebrewDir()

	updateCmd := "brew update"
	err := plumbing.ExecToStdout(updateCmd, brewDir)
	if err != nil {
		return fmt.Errorf("Couldn't update brew: %v", err)
	}

	pkgVersion := ""
	if bt.version != "" {
		pkgVersion = fmt.Sprintf("@%s", bt.version)
	}

	log.Infof(ctx, "Going to install %s%s from Homebrew...", bt.pkgName, pkgVersion)
	installCmd := fmt.Sprintf("brew install %s%s", bt.pkgName, pkgVersion)
	err = plumbing.ExecToStdout(installCmd, brewDir)

	if err != nil {
		return fmt.Errorf("Couldn't intall %s@%s from  Homebrew: %v", bt.pkgName, bt.version, err)
	}

	return nil
}

func (bt homebrewBuildTool) setup(ctx context.Context) error {
	if bt.isPackage() {
		prefixPath, err := bt.packagePrefix(ctx, bt.packageVersionString())
		if err != nil {
			return fmt.Errorf("Unable to determine prefix for package %s: %v", bt.packageVersionString(), err)
		}
		binDir := filepath.Join(prefixPath, "bin")
		sbinDir := filepath.Join(prefixPath, "sbin")

		plumbing.PrependToPath(binDir)
		plumbing.PrependToPath(sbinDir)
	} else {
		brewDir := bt.homebrewDir()
		brewBinDir := filepath.Join(brewDir, "bin")
		plumbing.PrependToPath(brewBinDir)
		brewLibDir := filepath.Join(brewDir, "lib")
		os.Setenv("LD_LIBRARY_PATH", brewLibDir)
	}
	return nil
}
