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

type RubyBuildTool struct {
	BuildTool
	_version string
}

func NewRubyBuildTool(toolSpec string) RubyBuildTool {
	parts := strings.Split(toolSpec, ":")
	version := parts[1]

	tool := RubyBuildTool{
		_version: version,
	}

	return tool
}

func (bt RubyBuildTool) Version() string {
	return bt._version
}

func (bt RubyBuildTool) RubyDir() string { 
	return filepath.Join(ToolsDir(), "rbenv", "versions", bt.Version())
}

/*
TODO: Install libssl-dev (or equivalent / warn) and zlib-dev based on platform
*/
func (bt RubyBuildTool) Install() error {
	toolsDir := ToolsDir()

	rbenvGitUrl := "https://github.com/rbenv/rbenv.git"
	rbenvDir := filepath.Join(toolsDir, "rbenv")
	rubyVersionDir := filepath.Join(rbenvDir, "versions", bt.Version())


	bt.InstallPlatformDependencies()

	if _, err := os.Stat(rbenvDir); err == nil {
		fmt.Printf("rbenv installed in %s\n", rbenvDir)
	} else {
		fmt.Printf("Installing rbenv\n")

		_, err := git.PlainClone(rbenvDir, false, &git.CloneOptions{
			URL:      rbenvGitUrl,
			Progress: os.Stdout,
		})

		if err != nil {
			fmt.Printf("Unable to clone rbenv!\n")
			return fmt.Errorf("Couldn't clone rbenv: %v\n", err)
		}
	}

	pluginsDir := filepath.Join(rbenvDir, "plugins")
	MkdirAsNeeded(pluginsDir)

	rubyBuildGitUrl := "https://github.com/rbenv/ruby-build.git"
	rubyBuildDir := filepath.Join(pluginsDir, "ruby-build")

	if PathExists(rubyBuildDir) {
		fmt.Printf("ruby-build installed in %s\n", rubyBuildDir)
        } else {
		fmt.Printf("Installing ruby-build\n")

		_, err := git.PlainClone(rubyBuildDir, false, &git.CloneOptions{
			URL:      rubyBuildGitUrl,
			Progress: os.Stdout,
		})

		if err != nil {
			fmt.Printf("Unable to clone ruby-build!\n")
			return fmt.Errorf("Couldn't clone ruby-build: %v\n", err)
		}
	}	

	if _, err := os.Stat(rubyVersionDir); err == nil {
		fmt.Printf("Ruby %s installed in %s\n", bt.Version(), rubyVersionDir)
	} else {
		os.Setenv("RBENV_ROOT", rbenvDir)
		PrependToPath(filepath.Join(rbenvDir, "bin"))

		installCmd := fmt.Sprintf("rbenv install %s", bt.Version())
		ExecToStdout(installCmd, rbenvDir)
	}

	return nil
}

func (bt RubyBuildTool) Setup() error {
	workspace := LoadWorkspace()
	gemsDir := filepath.Join(workspace.BuildRoot(), "rubygems")
	MkdirAsNeeded(gemsDir)

	fmt.Printf("Setting GEM_HOME to %s\n", gemsDir)
	os.Setenv("GEM_HOME", gemsDir)

	rubyDir := bt.RubyDir()
	PrependToPath(filepath.Join(rubyDir, "bin"))



	return nil

}

func (bt RubyBuildTool) InstallPlatformDependencies() error { 
	gi := goInfo.GetInfo()
	if gi.GoOS == "darwin" { 
		if strings.HasPrefix(gi.Core, "18.") {
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
