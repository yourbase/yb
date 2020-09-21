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

	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/types"
	"gopkg.in/src-d/go-git.v4"
)

const YBRubyDownloadTemplate = "https://yourbase-build-tools.s3-us-west-2.amazonaws.com/ruby/ruby-{{ .Version }}-{{ .OS }}-{{ .Arch }}-{{ .OsVersion }}.{{ .Extension }}"

func (bt RubyBuildTool) DownloadUrl() string {
	extension := "tar.bz2"
	osVersion := OSVersion()
	arch := Arch()

	if arch == "amd64" {
		arch = "x86_64"
	}

	operatingSystem := OS()
	if operatingSystem == "darwin" {
		operatingSystem = "Darwin"
	}

	if operatingSystem == "linux" {
		operatingSystem = "Linux"
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

	url, err := plumbing.TemplateToString(YBRubyDownloadTemplate, data)

	if err != nil {
		log.Infof("Error generating download URL: %v", err)
	}

	return url
}

type RubyBuildTool struct {
	types.BuildTool
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
		log.Infof("Ruby %s installed in %s", bt.Version(), rubyVersionDir)
	} else {

		if bt.binaryExists() {
			rubyVersionsDir := bt.versionsDir()
			downloadUrl := bt.DownloadUrl()
			log.Infof("Will download pre-built Ruby from %s", downloadUrl)

			localFile, err := plumbing.DownloadFileWithCache(downloadUrl)
			if err != nil {
				log.Errorf("Unable to download: %v", err)
				return err
			}
			err = archiver.Unarchive(localFile, rubyVersionsDir)
			if err != nil {
				log.Errorf("Unable to decompress: %v", err)
				return err
			}

			return nil
		} else {
			log.Infof("Couldn't find a file at %s...", bt.DownloadUrl())
		}

		rbenvGitUrl := "https://github.com/rbenv/rbenv.git"
		rbenvDir := bt.rbenvDir()

		bt.InstallPlatformDependencies()

		if _, err := os.Stat(rbenvDir); err == nil {
			log.Infof("rbenv installed in %s", rbenvDir)
		} else {
			log.Infof("Installing rbenv")

			_, err := git.PlainClone(rbenvDir, false, &git.CloneOptions{
				URL:      rbenvGitUrl,
				Progress: os.Stdout,
			})

			if err != nil {
				log.Infof("Unable to clone rbenv!")
				return fmt.Errorf("Couldn't clone rbenv: %v", err)
			}
		}

		pluginsDir := filepath.Join(rbenvDir, "plugins")
		plumbing.MkdirAsNeeded(pluginsDir)

		rubyBuildGitUrl := "https://github.com/rbenv/ruby-build.git"
		rubyBuildDir := filepath.Join(pluginsDir, "ruby-build")

		if plumbing.PathExists(rubyBuildDir) {
			log.Infof("ruby-build installed in %s", rubyBuildDir)
		} else {
			log.Infof("Installing ruby-build")

			_, err := git.PlainClone(rubyBuildDir, false, &git.CloneOptions{
				URL:      rubyBuildGitUrl,
				Progress: os.Stdout,
			})

			if err != nil {
				log.Errorf("Unable to clone ruby-build!")
				return fmt.Errorf("Couldn't clone ruby-build: %v", err)
			}
		}

		os.Setenv("RBENV_ROOT", rbenvDir)
		plumbing.PrependToPath(filepath.Join(rbenvDir, "bin"))

		installCmd := fmt.Sprintf("rbenv install %s", bt.Version())
		plumbing.ExecToStdout(installCmd, rbenvDir)
	}

	return nil
}

func (bt RubyBuildTool) Setup() error {
	gemsDir := filepath.Join(bt.spec.PackageCacheDir, "rubygems")
	plumbing.MkdirAsNeeded(gemsDir)

	log.Infof("Setting GEM_HOME to %s", gemsDir)
	os.Setenv("GEM_HOME", gemsDir)

	gemBinDir := filepath.Join(gemsDir, "bin")

	rubyDir := bt.RubyDir()
	plumbing.PrependToPath(filepath.Join(rubyDir, "bin"))
	plumbing.PrependToPath(gemBinDir)

	return nil
}

func (bt RubyBuildTool) InstallPlatformDependencies() error {
	gi := goInfo.GetInfo()
	if gi.GoOS == "darwin" {
		if strings.HasPrefix(gi.Core, "18.") {
			// Need to install the headers on Mojave
			if !plumbing.PathExists("/usr/include/zlib.h") {
				installCmd := "sudo -S installer -pkg /Library/Developer/CommandLineTools/Packages/macOS_SDK_headers_for_macOS_10.14.pkg -target /"
				log.Infof("Going to run: %s", installCmd)
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
