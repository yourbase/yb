package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"os/exec"

	"gopkg.in/src-d/go-git.v4"
	"github.com/matishsiao/goInfo"
)

type PythonBuildTool struct {
	BuildTool
	_version string
}

func NewPythonBuildTool(toolSpec string) PythonBuildTool {
	parts := strings.Split(toolSpec, ":")
	version := parts[1]

	tool := PythonBuildTool{
		_version: version,
	}

	return tool
}

func (bt PythonBuildTool) Version() string {
	return bt._version
}

/*
TODO: Install libssl-dev (or equivalent / warn) and zlib-dev based on platform
*/
func (bt PythonBuildTool) Install() error {
	workspace := LoadWorkspace()
	buildDir := workspace.BuildRoot()
	toolsDir := ToolsDir()

	pyenvGitUrl := "https://github.com/pyenv/pyenv.git"
	pyenvDir := filepath.Join(toolsDir, "pyenv")
	pythonVersionDir := filepath.Join(pyenvDir, "versions", bt.Version())

	virtualenvDir := filepath.Join(buildDir, "python", bt.Version())

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
		fmt.Printf("Python %s installed in %s", bt.Version(), pythonVersionDir)
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
		pythonBinPath := filepath.Join(pythonVersionDir, "bin", "python")
		cmd := fmt.Sprintf("virtualenv -p %s %s", pythonBinPath, virtualenvDir)
		ExecToStdout(cmd, buildDir)
	}

	return nil
}

func (bt PythonBuildTool) Setup() error {

	workspace := LoadWorkspace()
	buildDir := workspace.BuildRoot()
	virtualenvDir := filepath.Join(buildDir, "python", bt.Version())

	PrependToPath(filepath.Join(virtualenvDir, "bin"))

	return nil

}

func (bt PythonBuildTool) InstallPlatformDependencies() error { 
	gi := goInfo.GetInfo()
	if gi.GoOS == "darwin" { 
		if gi.Core == "18.2.0" {
			// Need to install the headers on Mojave
			if !PathExists("/usr/include/zlib.h") {
				installCmd := "sudo installer -pkg /Library/Developer/CommandLineTools/Packages/macOS_SDK_headers_for_macOS_10.14.pkg -target /"
				fmt.Println("Going to run: %s\n", installCmd)
				cmdArgs := strings.Split(installCmd, " ")
				cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
				cmd.Stdout = os.Stdout
				cmd.Stdin = os.Stdin
				cmd.Stderr = os.Stderr
				cmd.Run()
			}
		}
	}

	return nil
}
