package buildpacks

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/yourbase/yb/plumbing"
	"zombiezen.com/go/log"
)

const (
	anacondaToolVersion = "4.8.3"
	// The version above needs a newer template
	anacondaURLTemplate = "https://repo.continuum.io/miniconda/Miniconda{{.PyNum}}-{{.Version}}-{{.OS}}-{{.Arch}}.{{.Extension}}"
)

type pythonBuildTool struct {
	version string
	spec    buildToolSpec
}

func newPythonBuildTool(toolSpec buildToolSpec) pythonBuildTool {
	tool := pythonBuildTool{
		version: toolSpec.version,
		spec:    toolSpec,
	}

	return tool
}

func (bt pythonBuildTool) anacondaInstallDir() string {
	return filepath.Join(bt.spec.sharedCacheDir, "miniconda3", fmt.Sprintf("miniconda-%s", anacondaToolVersion))
}

func (bt pythonBuildTool) environmentDir() string {
	return filepath.Join(bt.spec.packageCacheDir, "conda-python", bt.version)
}

func (bt pythonBuildTool) install(ctx context.Context) error {
	anacondaDir := bt.anacondaInstallDir()
	setupDir := bt.spec.packageDir

	if _, err := os.Stat(anacondaDir); err == nil {
		log.Infof(ctx, "anaconda installed in %s", anacondaDir)
	} else {
		log.Infof(ctx, "Installing anaconda")

		downloadURL := bt.downloadURL()

		log.Infof(ctx, "Downloading Miniconda from URL %s...", downloadURL)
		localFile, err := plumbing.DownloadFileWithCache(ctx, http.DefaultClient, downloadURL)
		if err != nil {
			log.Errorf(ctx, "Unable to download: %v", err)
			return err
		}

		// TODO: Windows
		for _, cmd := range []string{
			fmt.Sprintf("chmod +x %s", localFile),
			fmt.Sprintf("bash %s -b -p %s", localFile, anacondaDir),
		} {
			log.Infof(ctx, "Running: '%v' ", cmd)
			plumbing.ExecToStdout(cmd, setupDir)
		}

	}

	return nil
}

func (bt pythonBuildTool) downloadURL() string {
	opsys := OS()
	arch := Arch()
	extension := "sh"
	version := bt.version

	if version == "" {
		version = "latest"
	}

	if arch == "amd64" {
		arch = "x86_64"
	}

	if opsys == "darwin" {
		opsys = "MacOSX"
	}

	if opsys == "linux" {
		opsys = "Linux"
	}

	if opsys == "windows" {
		opsys = "Windows"
		extension = "exe"
	}

	data := struct {
		PyNum     int
		OS        string
		Arch      string
		Version   string
		Extension string
	}{
		3,
		opsys,
		arch,
		anacondaToolVersion,
		extension,
	}

	url, _ := plumbing.TemplateToString(anacondaURLTemplate, data)

	return url
}

func (bt pythonBuildTool) setup(ctx context.Context) error {
	condaDir := bt.anacondaInstallDir()
	envDir := bt.environmentDir()

	if _, err := os.Stat(envDir); err == nil {
		log.Infof(ctx, "environment installed in %s", envDir)
	} else {
		currentPath := os.Getenv("PATH")
		newPath := fmt.Sprintf("PATH=%s:%s", filepath.Join(condaDir, "bin"), currentPath)
		setupDir := bt.spec.packageDir
		condaBin := filepath.Join(condaDir, "bin", "conda")

		for _, cmd := range []string{
			fmt.Sprintf("%s install -c anaconda setuptools", condaBin),
			fmt.Sprintf("%s config --set always_yes yes --set changeps1 no", condaBin),
			fmt.Sprintf("%s update -q conda", condaBin),
			fmt.Sprintf("%s create --prefix %s python=%s", condaBin, envDir, bt.version),
		} {
			log.Infof(ctx, "Running: '%v' ", cmd)
			if err := plumbing.ExecToStdoutWithEnv(cmd, setupDir, []string{newPath}); err != nil {
				log.Errorf(ctx, "Unable to run setup command: %s", cmd)
				return fmt.Errorf("Unable to run '%s': %v", cmd, err)
			}
		}
	}

	// Add new env to path
	plumbing.PrependToPath(filepath.Join(envDir, "bin"))

	return nil

}
