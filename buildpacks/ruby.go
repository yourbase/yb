package buildpacks

import (
	"context"
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
	"gopkg.in/src-d/go-git.v4"
)

const rubyDownloadTemplate = "https://yourbase-build-tools.s3-us-west-2.amazonaws.com/ruby/ruby-{{ .Version }}-{{ .OS }}-{{ .Arch }}-{{ .OsVersion }}.{{ .Extension }}"

func (bt rubyBuildTool) downloadURL() string {
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
		bt.version,
		extension,
	}

	url, err := plumbing.TemplateToString(rubyDownloadTemplate, data)

	if err != nil {
		log.Infof("Error generating download URL: %v", err)
	}

	return url
}

type rubyBuildTool struct {
	version string
	spec    buildToolSpec
}

func newRubyBuildTool(toolSpec buildToolSpec) rubyBuildTool {
	tool := rubyBuildTool{
		version: toolSpec.version,
		spec:    toolSpec,
	}

	return tool
}

func (bt rubyBuildTool) versionsDir() string {
	return filepath.Join(bt.rbenvDir(), "versions")
}

func (bt rubyBuildTool) rubyDir() string {
	return filepath.Join(bt.rbenvDir(), "versions", bt.version)
}

func (bt rubyBuildTool) rbenvDir() string {
	return filepath.Join(bt.spec.sharedCacheDir, "rbenv")
}

func (bt rubyBuildTool) binaryExists() bool {
	downloadUrl := bt.downloadURL()
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
func (bt rubyBuildTool) install(ctx context.Context) error {

	rubyVersionDir := bt.rubyDir()

	if _, err := os.Stat(rubyVersionDir); err == nil {
		log.Infof("Ruby %s installed in %s", bt.version, rubyVersionDir)
	} else {

		if bt.binaryExists() {
			rubyVersionsDir := bt.versionsDir()
			downloadUrl := bt.downloadURL()
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
			log.Infof("Couldn't find a file at %s...", bt.downloadURL())
		}

		rbenvGitUrl := "https://github.com/rbenv/rbenv.git"
		rbenvDir := bt.rbenvDir()

		bt.installPlatformDependencies()

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
		if err := os.MkdirAll(pluginsDir, 0777); err != nil {
			return fmt.Errorf("install Ruby: %w", err)
		}

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

		installCmd := fmt.Sprintf("rbenv install %s", bt.version)
		plumbing.ExecToStdout(installCmd, rbenvDir)
	}

	return nil
}

func (bt rubyBuildTool) setup(ctx context.Context) error {
	gemsDir := filepath.Join(bt.spec.packageCacheDir, "rubygems")
	if err := os.MkdirAll(gemsDir, 0777); err != nil {
		return fmt.Errorf("install Ruby: %w", err)
	}

	log.Infof("Setting GEM_HOME to %s", gemsDir)
	os.Setenv("GEM_HOME", gemsDir)

	gemBinDir := filepath.Join(gemsDir, "bin")

	rubyDir := bt.rubyDir()
	plumbing.PrependToPath(filepath.Join(rubyDir, "bin"))
	plumbing.PrependToPath(gemBinDir)

	return nil
}

func (bt rubyBuildTool) installPlatformDependencies() error {
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
