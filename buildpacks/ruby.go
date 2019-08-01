package buildpacks

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/johnewart/archiver"
	"github.com/matishsiao/goInfo"

	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
	"gopkg.in/src-d/go-git.v4"
)

const YBRubyDownloadTemplate = "https://yourbase-build-tools.s3-us-west-2.amazonaws.com/ruby/ruby-{{ .Version }}-{{ .OS }}-{{ .Arch }}-{{ .OsVersion }}.{{ .Extension }}"

func (bt RubyBuildTool) DownloadUrl() string {
	extension := "tar.bz2"
	osVersion := ""
	arch := Arch()

	if arch == "amd64" {
		arch = "x86_64"
	}

	operatingSystem := OS()
	if operatingSystem == "darwin" {
		operatingSystem = "Darwin"
		osVersion = "18.5.0"
	}

	if operatingSystem == "windows" {
		extension = "zip"
	}

	data := struct {
		OS        string
		OsVersion string
		Arch      string
		Version   string
		Extension string
	}{
		operatingSystem,
		osVersion,
		arch,
		bt.Version(),
		extension,
	}

	url, err := TemplateToString(YBRubyDownloadTemplate, data)

	if err != nil {
		fmt.Printf("Error generating download URL: %v\n", err)
	}

	return url
}

type RubyBuildTool struct {
	BuildTool
	version string
	spec    BuildToolSpec
}

func NewRubyBuildTool(toolSpec BuildToolSpec) RubyBuildTool {
	tool := RubyBuildTool{
		version: toolSpec.Version,
		spec:    toolSpec,
	}

	return tool
}

func (bt RubyBuildTool) Version() string {
	return bt.version
}

func (bt RubyBuildTool) versionsDir() string {
	return filepath.Join(bt.rbenvDir(), "versions")
}

func (bt RubyBuildTool) RubyDir() string {
	return filepath.Join(bt.rbenvDir(), "versions", bt.Version())
}

func (bt RubyBuildTool) rbenvDir() string {
	return filepath.Join(bt.spec.SharedCacheDir, "rbenv")
}

func (bt RubyBuildTool) binaryExists() bool {
	downloadUrl := bt.DownloadUrl()
	resp, err := http.Head(downloadUrl)

	if err != nil {
		return false
	}

	if resp.StatusCode == 200 {
		return true
	}

	return false
}

/*
TODO: Install libssl-dev (or equivalent / warn) and zlib-dev based on platform
*/
func (bt RubyBuildTool) Install() error {

	rubyVersionDir := bt.RubyDir()

	if _, err := os.Stat(rubyVersionDir); err == nil {
		fmt.Printf("Ruby %s installed in %s\n", bt.Version(), rubyVersionDir)
	} else {

		if bt.binaryExists() {
			rubyVersionsDir := bt.versionsDir()
			downloadUrl := bt.DownloadUrl()
			fmt.Printf("Will download pre-built Ruby from %s\n", downloadUrl)

			localFile, err := DownloadFileWithCache(downloadUrl)
			if err != nil {
				fmt.Printf("Unable to download: %v\n", err)
				return err
			}
			err = archiver.Unarchive(localFile, rubyVersionsDir)
			if err != nil {
				fmt.Printf("Unable to decompress: %v\n", err)
				return err
			}

			return nil
		} else {
			fmt.Printf("Couldn't find a file at %s...\n", bt.DownloadUrl())
		}

		rbenvGitUrl := "https://github.com/rbenv/rbenv.git"
		rbenvDir := bt.rbenvDir()

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

		os.Setenv("RBENV_ROOT", rbenvDir)
		PrependToPath(filepath.Join(rbenvDir, "bin"))

		installCmd := fmt.Sprintf("rbenv install %s", bt.Version())
		ExecToStdout(installCmd, rbenvDir)
	}

	return nil
}

func (bt RubyBuildTool) Setup() error {
	gemsDir := filepath.Join(bt.spec.PackageCacheDir, "rubygems")
	MkdirAsNeeded(gemsDir)

	fmt.Printf("Setting GEM_HOME to %s\n", gemsDir)
	os.Setenv("GEM_HOME", gemsDir)

	gemBinDir := filepath.Join(gemsDir, "bin")

	rubyDir := bt.RubyDir()
	PrependToPath(filepath.Join(rubyDir, "bin"))
	PrependToPath(gemBinDir)

	return nil
}

func (bt RubyBuildTool) InstallPlatformDependencies() error {
	gi := goInfo.GetInfo()
	if gi.GoOS == "darwin" {
		if strings.HasPrefix(gi.Core, "18.") {
			// Need to install the headers on Mojave
			if !PathExists("/usr/include/zlib.h") {
				installCmd := "sudo -S installer -pkg /Library/Developer/CommandLineTools/Packages/macOS_SDK_headers_for_macOS_10.14.pkg -target /"
				fmt.Printf("Going to run: %s\n", installCmd)
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
