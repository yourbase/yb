package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
)

const (
	anacondaToolVersion = "4.8.3"
	// The version above needs a newer template
	anacondaURLTemplate = "https://repo.continuum.io/miniconda/Miniconda{{.PyNum}}-py37_{{.Version}}-{{.OS}}-{{.Arch}}.{{.Extension}}"
)

type PythonBuildTool struct {
	version string
	spec    BuildToolSpec
}

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

func (bt PythonBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	anacondaDir := filepath.Join(t.ToolsDir(ctx), "miniconda3", "miniconda-"+anacondaToolVersion)
	setupDir := bt.spec.PackageDir

	if t.PathExists(ctx, anacondaDir) {
		log.Infof("anaconda installed in %s", anacondaDir)
		return anacondaDir, nil
	}
	log.Infof("Installing anaconda")

	downloadURL, err := bt.DownloadURL(ctx)
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}

	log.Infof("Downloading Miniconda from URL %s...", downloadURL)
	localFile, err := t.DownloadFile(ctx, downloadURL)
	if err != nil {
		log.Errorf("Unable to download: %v", err)
		return "", err
	}

	// TODO: Windows
	for _, cmd := range []string{
		fmt.Sprintf("chmod +x %s", localFile),
		fmt.Sprintf("bash %s -b -p %s", localFile, anacondaDir),
	} {
		log.Infof("Running: '%v' ", cmd)
		p := runtime.Process{
			Command:   cmd,
			Directory: setupDir,
		}
		if err := t.Run(ctx, p); err != nil {
			return "", fmt.Errorf("installing python: %v", err)
		}
	}

	return anacondaDir, nil
}

func (bt PythonBuildTool) DownloadURL(ctx context.Context) (string, error) {
	opsys := ""
	arch := ""
	extension := "sh"
	version := bt.Version()

	if version == "" {
		version = "latest"
	}

	switch bt.spec.InstallTarget.Architecture() {
	case runtime.Amd64:
		arch = "x86_64"
	}

	switch bt.spec.InstallTarget.OS() {
	case runtime.Linux:
		opsys = "Linux"
	case runtime.Darwin:
		opsys = "MacOSX"
	case runtime.Windows:
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

	url, err := TemplateToString(anacondaURLTemplate, data)
	return url, err
}

func (bt PythonBuildTool) Setup(ctx context.Context, condaDir string) error {
	t := bt.spec.InstallTarget

	envDir := filepath.Join(t.ToolsDir(ctx), "conda-python", bt.Version())
	t.PrependToPath(ctx, filepath.Join(condaDir, "bin"))

	if t.PathExists(ctx, envDir) {
		log.Infof("environment installed in %s", envDir)
		return nil
	}
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
			Command:   cmd,
			Directory: setupDir,
		}

		if err := t.Run(ctx, p); err != nil {
			log.Errorf("Unable to run setup command: %s", cmd)
			return fmt.Errorf("running '%s': %v", cmd, err)
		}
	}

	// Add new env to path
	t.PrependToPath(ctx, filepath.Join(envDir, "bin"))

	return nil

}
