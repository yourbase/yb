package buildpacks

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
	. "github.com/yourbase/yb/types"
)

type PythonBuildTool struct {
	BuildTool
	version string
	spec    BuildToolSpec
}

var ANACONDA_URL_TEMPLATE = "https://repo.continuum.io/miniconda/Miniconda{{.PyNum}}-{{.Version}}-{{.OS}}-{{.Arch}}.{{.Extension}}"

const AnacondaToolVersion = "4.7.10"

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

func (bt PythonBuildTool) AnacondaInstallDir() string {
	return filepath.Join(bt.spec.SharedCacheDir, "miniconda3", fmt.Sprintf("miniconda-%s", AnacondaToolVersion))
}

func (bt PythonBuildTool) EnvironmentDir() string {
	return filepath.Join(bt.spec.PackageCacheDir, "conda-python", bt.Version())
}

func (bt PythonBuildTool) Install() error {
	anacondaDir := bt.AnacondaInstallDir()
	setupDir := bt.spec.PackageDir
	t := bt.spec.InstallTarget

	if _, err := os.Stat(anacondaDir); err == nil {
		log.Infof("anaconda installed in %s", anacondaDir)
	} else {
		log.Infof("Installing anaconda")

		downloadUrl := bt.DownloadUrl()

		log.Infof("Downloading Miniconda from URL %s...", downloadUrl)
		localFile, err := t.DownloadFile(downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err
		}

		// TODO: Windows
		for _, cmd := range []string{
			fmt.Sprintf("chmod +x %s", localFile),
			fmt.Sprintf("bash %s -b -p %s", localFile, anacondaDir),
		} {
			log.Infof("Running: '%v' ", cmd)
			p := runtime.Process{
				Command: cmd,
				Directory: setupDir,
			}
			if err := t.Run(p); err != nil {
				return fmt.Errorf("Couldn't install python: %v", err)
			}
		}

	}

	return nil
}

func (bt PythonBuildTool) DownloadUrl() string {
	opsys := OS()
	arch := Arch()
	extension := "sh"
	version := bt.Version()

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
		AnacondaToolVersion,
		extension,
	}

	url, _ := TemplateToString(ANACONDA_URL_TEMPLATE, data)

	return url
}

func (bt PythonBuildTool) Setup() error {
	condaDir := bt.AnacondaInstallDir()
	envDir := bt.EnvironmentDir()
	t := bt.spec.InstallTarget

	if _, err := os.Stat(envDir); err == nil {
		log.Infof("environment installed in %s", envDir)
	} else {
		currentPath := os.Getenv("PATH")
		newPath := fmt.Sprintf("PATH=%s:%s", filepath.Join(condaDir, "bin"), currentPath)
		setupDir := bt.spec.PackageDir
		condaBin := filepath.Join(condaDir, "bin", "conda")

		for _, cmd := range []string{
			fmt.Sprintf("%s install -c anaconda setuptools", condaBin),
			fmt.Sprintf("%s config --set always_yes yes --set changeps1 no", condaBin),
			fmt.Sprintf("%s update -q conda", condaBin),
			fmt.Sprintf("%s create --prefix %s python=%s", condaBin, envDir, bt.Version()),
		} {
			log.Infof("Running: '%v' ", cmd)
			p := runtime.Process{
				Command:     cmd,
				Interactive: false,
				Directory:   setupDir,
				Environment: []string{newPath},
			}

			if err := t.Run(p); err != nil {
				log.Errorf("Unable to run setup command: %s", cmd)
				return fmt.Errorf("Unable to run '%s': %v", cmd, err)
			}
		}
	}

	// Add new env to path
	t.PrependToPath(filepath.Join(envDir, "bin"))

	return nil

}
