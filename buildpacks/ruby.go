package buildpacks

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"

	"gopkg.in/src-d/go-git.v4"
)

const rubyDownloadTemplate = "https://yourbase-build-tools.s3-us-west-2.amazonaws.com/ruby/ruby-{{ .Version }}-{{ .OS }}-{{ .Arch }}-{{ .OsVersion }}.{{ .Extension }}"

func (bt RubyBuildTool) DownloadURL(ctx context.Context) (string, error) {
	extension := "tar.bz2"
	operatingSystem := "unknown"
	osVersion := bt.spec.InstallTarget.OSVersion(ctx)
	arch := Arch()

	if arch == "amd64" {
		arch = "x86_64"
	}

	switch bt.spec.InstallTarget.OS() {
	case runtime.Linux:
		operatingSystem = "Linux"
	case runtime.Darwin:
		operatingSystem = "Darwin"
	case runtime.Windows:
		operatingSystem = "windows"
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

	url, err := TemplateToString(rubyDownloadTemplate, data)
	return url, err
}

type RubyBuildTool struct {
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

func (bt RubyBuildTool) binaryExists(downloadURL string) bool {
	resp, err := http.Head(downloadURL)

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
func (bt RubyBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	rbenvDir := filepath.Join(t.ToolsDir(ctx), "rbenv")
	rubyVersionsDir := filepath.Join(rbenvDir, "versions")
	rubyVersionDir := filepath.Join(rubyVersionsDir, bt.Version())

	if t.PathExists(ctx, rubyVersionDir) {
		log.Infof("Ruby %s installed in %s", bt.Version(), rubyVersionDir)
		return rubyVersionDir, nil
	}

	downloadURL, err := bt.DownloadURL(ctx)
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}
	if bt.binaryExists(downloadURL) {
		log.Infof("Will download pre-built Ruby from %s", downloadURL)

		localFile, err := t.DownloadFile(ctx, downloadURL)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return "", err
		}
		err = t.Unarchive(ctx, localFile, rubyVersionsDir)
		if err != nil {
			log.Errorf("Unable to decompress: %v", err)
			return "", err
		}

		return rubyVersionDir, nil
	} else {
		log.Infof("Couldn't find a file at %s...", downloadURL)
	}

	rbenvGitUrl := "https://github.com/rbenv/rbenv.git"

	if t.PathExists(ctx, rbenvDir) {
		log.Infof("rbenv installed in %s", rbenvDir)
	} else {
		log.Infof("Installing rbenv")

		_, err := git.PlainClone(rbenvDir, false, &git.CloneOptions{
			URL:      rbenvGitUrl,
			Progress: os.Stdout,
		})

		if err != nil {
			log.Infof("Unable to clone rbenv!")
			return "", fmt.Errorf("Couldn't clone rbenv: %v", err)
		}
	}

	pluginsDir := filepath.Join(rbenvDir, "plugins")
	t.MkdirAsNeeded(ctx, pluginsDir)

	rubyBuildGitUrl := "https://github.com/rbenv/ruby-build.git"
	rubyBuildDir := filepath.Join(pluginsDir, "ruby-build")

	if t.PathExists(ctx, rubyBuildDir) {
		log.Infof("ruby-build installed in %s", rubyBuildDir)
	} else {
		log.Infof("Installing ruby-build")

		_, err := git.PlainClone(rubyBuildDir, false, &git.CloneOptions{
			URL:      rubyBuildGitUrl,
			Progress: os.Stdout,
		})

		if err != nil {
			log.Errorf("Unable to clone ruby-build!")
			return "", fmt.Errorf("Couldn't clone ruby-build: %v", err)
		}
	}

	t.SetEnv("RBENV_ROOT", rbenvDir)
	t.PrependToPath(ctx, filepath.Join(rbenvDir, "bin"))

	p := runtime.Process{
		Command:   "rbenv install " + bt.Version(),
		Directory: rbenvDir,
	}
	err = t.Run(ctx, p)
	if err != nil {
		return "", err
	}

	return rubyVersionDir, nil
}

func (bt RubyBuildTool) Setup(ctx context.Context, rubyDir string) error {
	t := bt.spec.InstallTarget

	gemsDir := filepath.Join(t.ToolsDir(ctx), "rubygems")

	log.Infof("Setting GEM_HOME to %s", gemsDir)
	t.SetEnv("GEM_HOME", gemsDir)

	gemBinDir := filepath.Join(gemsDir, "bin")

	t.PrependToPath(ctx, filepath.Join(rubyDir, "bin"))
	t.PrependToPath(ctx, gemBinDir)

	return nil
}

// TODO When we have another type of isolation mechanism, change this to support it
/*func (bt RubyBuildTool) InstallPlatformDependencies() error {
	gi := goInfo.GetInfo()
	if gi.GoOS == "darwin" {
		if strings.HasPrefix(gi.Core, "18.") {
			// Need to install the headers on Mojave
			if !PathExists("/usr/include/zlib.h") {
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
}*/
