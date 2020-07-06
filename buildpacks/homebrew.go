package buildpacks

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/matishsiao/goInfo"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
	"gopkg.in/src-d/go-git.v4"
)

type HomebrewBuildTool struct {
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

func (bt HomebrewBuildTool) PackagePrefix(ctx context.Context, packageString string) (string, error) {
	t := bt.spec.InstallTarget

	if bt.pkgPrefix != "" {
		return bt.pkgPrefix, nil
	}

	var buf bytes.Buffer
	bufWriter := bufio.NewWriter(&buf)

	p := runtime.Process{
		Command: "brew --prefix " + packageString,
		Output:  bufWriter,
	}

	err := t.Run(ctx, p)
	if err != nil {
		return "", fmt.Errorf("Couldn't get prefix for package %s: %v", bt.pkgName, err)
	}

	err = bufWriter.Flush()
	if err != nil {
		return "", fmt.Errorf("Couldn't get prefix for package %s: %v", bt.pkgName, err)
	}

	rd := bufio.NewReader(&buf)

	prefixPath, err := rd.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("Couldn't get prefix for package %s: %v", bt.pkgName, err)
	}
	prefixPath = strings.TrimSuffix(prefixPath, "\n")
	bt.pkgPrefix = prefixPath
	log.Debugf("Prefix for homebrew package %s is %s", bt.pkgName, prefixPath)

	return prefixPath, nil
}

func (bt HomebrewBuildTool) PackageInstalled(ctx context.Context) bool {
	prefix, err := bt.PackagePrefix(ctx, bt.PackageVersionString())
	t := bt.spec.InstallTarget
	if err != nil {
		return false
	}

	if prefix != "" {
		return t.PathExists(ctx, prefix)
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
func (bt HomebrewBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "homebrew")
	t.MkdirAsNeeded(ctx, installDir)

	// Normally we want to put this in the tools dir; for now we put it in the build dir because I'm not
	// sure how to handle installation of multiple versions of things via Brew so this will allow project-specific
	// versioning
	brewDir := filepath.Join(installDir, "brew")

	gi := goInfo.GetInfo()

	var err error
	switch gi.GoOS {
	case "darwin":
		err = bt.installDarwin(ctx, brewDir)
	case "linux":
		err = bt.installLinux(ctx, brewDir)
	default:
		err = fmt.Errorf("Unsupported platform: %s", gi.GoOS)
	}

	if err != nil {
		return "", fmt.Errorf("Unable to install Homebrew: %v", err)
	}

	if bt.IsPackage() {
		if bt.PackageInstalled(ctx) {
			log.Infof("Package %s already installed", bt.PackageVersionString())
		} else {
			log.Infof("Installing package %s", bt.PackageVersionString())
			err = bt.InstallPackage(ctx, brewDir)
			if err != nil {
				return "", err
			}
		}
	}

	return brewDir, nil
}

func (bt HomebrewBuildTool) InstallPackage(ctx context.Context, brewDir string) error {
	t := bt.spec.InstallTarget

	bt.Setup(ctx, brewDir)

	p := runtime.Process{
		Command:   "brew update",
		Directory: brewDir,
	}
	err := t.Run(ctx, p)
	if err != nil {
		return fmt.Errorf("Couldn't update brew: %v", err)
	}

	pkgVersion := ""
	if bt.version != "" {
		pkgVersion = fmt.Sprintf("@%s", bt.version)
	}

	log.Infof("Going to install %s%s from Homebrew...", bt.pkgName, pkgVersion)
	p.Command = "brew install " + bt.pkgName + pkgVersion
	err = t.Run(ctx, p)

	if err != nil {
		return fmt.Errorf("Couldn't intall %s@%s from  Homebrew: %v", bt.pkgName, bt.version, err)
	}

	return nil
}

func (bt HomebrewBuildTool) installDarwin(ctx context.Context, brewDir string) error {
	t := bt.spec.InstallTarget

	brewGitUrl := "https://github.com/Homebrew/brew.git"

	if t.PathExists(ctx, brewDir) {
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
	p := runtime.Process{
		Command: "brew update",
	}
	err := t.Run(ctx, p)
	if err != nil {
		return err
	}

	return nil
}

func (bt HomebrewBuildTool) installLinux(ctx context.Context, brewDir string) error {
	t := bt.spec.InstallTarget

	brewGitUrl := "https://github.com/Homebrew/brew.git"

	if t.PathExists(ctx, brewDir) {
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
		p := runtime.Process{
			Command: "brew update",
		}
		err = t.Run(ctx, p)
		if err != nil {
			return err
		}
	}
	return nil
}

func (bt HomebrewBuildTool) Setup(ctx context.Context, brewDir string) error {
	t := bt.spec.InstallTarget
	if bt.IsPackage() {
		prefixPath, err := bt.PackagePrefix(ctx, bt.PackageVersionString())
		if err != nil {
			return fmt.Errorf("Unable to determine prefix for package %s: %v", bt.PackageVersionString(), err)
		}
		binDir := filepath.Join(prefixPath, "bin")
		sbinDir := filepath.Join(prefixPath, "sbin")

		t.PrependToPath(ctx, binDir)
		t.PrependToPath(ctx, sbinDir)
	} else {
		brewBinDir := filepath.Join(brewDir, "bin")
		t.PrependToPath(ctx, brewBinDir)
		brewLibDir := filepath.Join(brewDir, "lib")
		t.SetEnv("LD_LIBRARY_PATH", brewLibDir)
	}
	return nil
}

func (bt HomebrewBuildTool) DownloadURL(ctx context.Context) (string, error) {
	// No-op
	return "", nil
}
