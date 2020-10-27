package buildpack

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

	"github.com/yourbase/yb/internal/ybdata"
	"github.com/yourbase/yb/plumbing"
	"gopkg.in/src-d/go-git.v4"
	"zombiezen.com/go/log"
)

const rubyDownloadTemplate = "https://yourbase-build-tools.s3-us-west-2.amazonaws.com/ruby/ruby-{{ .Version }}-{{ .OS }}-{{ .Arch }}-{{ .OsVersion }}.{{ .Extension }}"

func (bt rubyBuildTool) downloadURL(ctx context.Context) string {
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
		log.Infof(ctx, "Error generating download URL: %v", err)
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
	return filepath.Join(bt.spec.cacheDir, "rbenv")
}

func (bt rubyBuildTool) binaryExists(ctx context.Context) bool {
	downloadUrl := bt.downloadURL(ctx)
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
		log.Infof(ctx, "Ruby %s installed in %s", bt.version, rubyVersionDir)
	} else {

		if bt.binaryExists(ctx) {
			rubyVersionsDir := bt.versionsDir()
			downloadURL := bt.downloadURL(ctx)
			log.Infof(ctx, "Will download pre-built Ruby from %s", downloadURL)

			localFile, err := ybdata.DownloadFileWithCache(ctx, http.DefaultClient, bt.spec.dataDirs, downloadURL)
			if err != nil {
				log.Errorf(ctx, "Unable to download: %v", err)
				return err
			}
			err = archiver.Unarchive(localFile, rubyVersionsDir)
			if err != nil {
				log.Errorf(ctx, "Unable to decompress: %v", err)
				return err
			}

			return nil
		} else {
			log.Infof(ctx, "Couldn't find a file at %s...", bt.downloadURL(ctx))
		}

		rbenvGitUrl := "https://github.com/rbenv/rbenv.git"
		rbenvDir := bt.rbenvDir()

		bt.installPlatformDependencies(ctx)

		if _, err := os.Stat(rbenvDir); err == nil {
			log.Infof(ctx, "rbenv installed in %s", rbenvDir)
		} else {
			log.Infof(ctx, "Installing rbenv")

			_, err := git.PlainClone(rbenvDir, false, &git.CloneOptions{
				URL:      rbenvGitUrl,
				Progress: os.Stdout,
			})

			if err != nil {
				log.Infof(ctx, "Unable to clone rbenv!")
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
			log.Infof(ctx, "ruby-build installed in %s", rubyBuildDir)
		} else {
			log.Infof(ctx, "Installing ruby-build")

			_, err := git.PlainClone(rubyBuildDir, false, &git.CloneOptions{
				URL:      rubyBuildGitUrl,
				Progress: os.Stdout,
			})

			if err != nil {
				log.Errorf(ctx, "Unable to clone ruby-build!")
				return fmt.Errorf("Couldn't clone ruby-build: %v", err)
			}
		}

		os.Setenv("RBENV_ROOT", rbenvDir)
		plumbing.PrependToPath(filepath.Join(rbenvDir, "bin"))

		installCmd := fmt.Sprintf("rbenv install %s", bt.version)
		if err := plumbing.ExecToStdout(installCmd, rbenvDir); err != nil {
			log.Errorf(ctx, "Unable to install ruby!")
			return fmt.Errorf("Couldn't install ruby: %v", err)
		}
	}

	return nil
}

func (bt rubyBuildTool) setup(ctx context.Context) error {
	gemsDir := filepath.Join(bt.spec.cacheDir, "rubygems")
	if err := os.MkdirAll(gemsDir, 0777); err != nil {
		return fmt.Errorf("install Ruby: %w", err)
	}

	log.Infof(ctx, "Setting GEM_HOME to %s", gemsDir)
	os.Setenv("GEM_HOME", gemsDir)

	gemBinDir := filepath.Join(gemsDir, "bin")

	rubyDir := bt.rubyDir()
	plumbing.PrependToPath(filepath.Join(rubyDir, "bin"))
	plumbing.PrependToPath(gemBinDir)

	return nil
}

func (bt rubyBuildTool) installPlatformDependencies(ctx context.Context) error {
	gi := goInfo.GetInfo()
	if gi.GoOS == "darwin" {
		if strings.HasPrefix(gi.Core, "18.") {
			// Need to install the headers on Mojave
			if !plumbing.PathExists("/usr/include/zlib.h") {
				installCmd := "sudo -S installer -pkg /Library/Developer/CommandLineTools/Packages/macOS_SDK_headers_for_macOS_10.14.pkg -target /"
				log.Infof(ctx, "Going to run: %s", installCmd)
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
