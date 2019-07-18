package buildpacks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/matishsiao/goInfo"
	"gopkg.in/src-d/go-git.v4"

	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
)

type PythonBuildTool struct {
	BuildTool
	version string
	spec    BuildToolSpec
}

func NewPythonBuildTool(toolSpec BuildToolSpec) PythonBuildTool {
	tool := PythonBuildTool{
		version: toolSpec.Version,
		spec:    toolSpec,
	}

	return tool
}

func (bt PythonBuildTool) Version() string {
	return bt.version
}

func (bt PythonBuildTool) pyenvInstallDir() string {
	return filepath.Join(bt.spec.SharedCacheDir, "pyenv")
}

func (bt PythonBuildTool) pythonInstallDir() string {
	return filepath.Join(bt.pyenvInstallDir(), "versions", bt.Version())
}
func (bt PythonBuildTool) pkgVirtualEnvDir() string {
	return filepath.Join(bt.spec.PackageCacheDir, "python", bt.Version())
}

/*
TODO: Install libssl-dev (or equivalent / warn) and zlib-dev based on platform
*/
func (bt PythonBuildTool) Install() error {
	pyenvGitUrl := "https://github.com/pyenv/pyenv.git"
	pyenvDir := bt.pyenvInstallDir()
	pythonVersionDir := bt.pythonInstallDir()
	virtualenvDir := bt.pkgVirtualEnvDir()

	bt.InstallPlatformDependencies()

	if _, err := os.Stat(pyenvDir); err == nil {
		fmt.Printf("pyenv installed in %s\n", pyenvDir)
	} else {
		fmt.Printf("Installing pyenv\n")

		_, err := git.PlainClone(pyenvDir, false, &git.CloneOptions{
			URL:      pyenvGitUrl,
			Progress: os.Stdout,
		})

		if err != nil {
			fmt.Printf("Unable to clone pyenv!\n")
			return fmt.Errorf("Couldn't clone pyenv: %v\n", err)
		}
	}

	if _, err := os.Stat(pythonVersionDir); err == nil {
		fmt.Printf("Python %s installed in %s\n", bt.Version(), pythonVersionDir)
	} else {
		os.Setenv("PYENV_ROOT", pyenvDir)
		PrependToPath(filepath.Join(pyenvDir, "bin"))

		installCmd := fmt.Sprintf("pyenv install %s", bt.Version())
		ExecToStdout(installCmd, pyenvDir)
	}

	virtualenvBin := filepath.Join(pythonVersionDir, "bin", "virtualenv")
	if _, err := os.Stat(virtualenvBin); err == nil {
		fmt.Printf("Virtualenv binary already installed in %s\n", virtualenvBin)
	} else {
		fmt.Printf("Installing virtualenv for Python in %s\n", pythonVersionDir)

		shimsDir := filepath.Join(pyenvDir, "shims")

		os.Setenv("PYENV_ROOT", pyenvDir)
		os.Setenv("PYENV_SHELL", "sh")
		os.Setenv("PYENV_VERSION", bt.Version())

		PrependToPath(filepath.Join(pyenvDir, "bin"))
		PrependToPath(shimsDir)
		setupCmd := fmt.Sprintf("pyenv rehash")
		ExecToStdout(setupCmd, pyenvDir)

		cmd := "pip install virtualenv"
		ExecToStdout(cmd, pyenvDir)
	}

	if _, err := os.Stat(virtualenvDir); err == nil {
		fmt.Printf("Virtualenv for %s exists in %s\n", bt.Version(), virtualenvDir)
	} else {
		fmt.Println("Creating virtualenv...")
		MkdirAsNeeded(virtualenvDir)
		pythonBinPath := filepath.Join(pythonVersionDir, "bin", "python")
		cmd := fmt.Sprintf("%s -p %s %s", virtualenvBin, pythonBinPath, virtualenvDir)
		ExecToStdout(cmd, bt.spec.PackageDir)
	}

	return nil
}

func (bt PythonBuildTool) Setup() error {
	virtualenvDir := bt.pkgVirtualEnvDir()
	PrependToPath(filepath.Join(virtualenvDir, "bin"))

	return nil

}

func (bt PythonBuildTool) InstallPlatformDependencies() error {
	gi := goInfo.GetInfo()
	if gi.GoOS == "darwin" {
		if strings.HasPrefix(gi.Core, "18.") {
			// Need to install the headers on Mojave
			if !PathExists("/usr/include/zlib.h") {
				installCmd := "sudo -S installer -pkg /Library/Developer/CommandLineTools/Packages/macOS_SDK_headers_for_macOS_10.14.pkg -target /"
				fmt.Println("Going to run:", installCmd)
				cmdArgs := strings.Split(installCmd, " ")
				cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
				cmd.Stdout = os.Stdout
				cmd.Stdin = os.Stdin
				cmd.Stderr = os.Stderr
				cmd.Run()
			}
		}
		// Thanks to pablopunk, here:
		// https://github.com/pyenv/pyenv/issues/1066#issuecomment-511835306
		if strings.HasPrefix(gi.Core, "18.6") {
			os.Setenv("SDKROOT", "/Applications/Xcode.app/Contents/Developer/Platforms/MacOSX.platform/Developer/SDKs/MacOSX10.14.sdk")
			os.Setenv("MACOSX_DEPLOYMENT_TARGET", "10.14")
		}
	}

	return nil
}
