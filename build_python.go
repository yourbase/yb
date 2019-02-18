package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/src-d/go-git.v4"
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
	buildDir := fmt.Sprintf("%s/build", workspace.Path)

	pyenvGitUrl := "https://github.com/pyenv/pyenv.git"
	pyenvDir := filepath.Join(buildDir, "pyenv")
	pythonVersionDir := filepath.Join(pyenvDir, "versions", bt.Version())

	if _, err := os.Stat(pyenvDir); err == nil {
		fmt.Printf("pyenv installed in %s", pyenvDir)
	} else {
		fmt.Printf("Installing pyenv\n")

		_, err := git.PlainClone(pyenvDir, false, &git.CloneOptions{
			URL:      pyenvGitUrl,
			Progress: os.Stdout,
		})

		if err != nil {
			fmt.Printf("Unable to clone pyenv!\n")
			return fmt.Errorf("Couldn't clond pyenv: %v\n", err)
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
	return nil
}

func (bt PythonBuildTool) Setup() error {

	workspace := LoadWorkspace()
	buildDir := fmt.Sprintf("%s/build", workspace.Path)
	pyenvDir := filepath.Join(buildDir, "pyenv")
	shimsDir := filepath.Join(pyenvDir, "shims")

	os.Setenv("PYENV_ROOT", pyenvDir)
	os.Setenv("PYENV_SHELL", "sh")
	os.Setenv("PYENV_VERSION", bt.Version())
	PrependToPath(filepath.Join(pyenvDir, "bin"))
	PrependToPath(shimsDir)

	setupCmd := fmt.Sprintf("pyenv rehash")
	ExecToStdout(setupCmd, pyenvDir)

	return nil

}
