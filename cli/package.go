package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnewart/archiver"
	"github.com/johnewart/subcommands"

	"github.com/yourbase/narwhal"
	. "github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/workspace"
)

type PackageCmd struct {
	target           string
	dockerRepository string
	dockerTag        string
}

func (*PackageCmd) Name() string     { return "package" }
func (*PackageCmd) Synopsis() string { return "Create a package artifact" }
func (*PackageCmd) Usage() string {
	return ``
}

func (p *PackageCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.target, "target", "", "Target package, if not the default")
	f.StringVar(&p.dockerRepository, "repository", "docker-registry.yourbase.io", "Repository base name")
	f.StringVar(&p.dockerTag, "tag", "latest", "Container tag")
}

/*
Executing the target involves:
1. Map source into the target container
2. Run any dependent components
3. Start target
*/
func (b *PackageCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	targetPackage := Package{}
	workspace, err := LoadWorkspace()

	if err == nil {
		if b.target != "" {
			targetPackage, _ = workspace.PackageByName(b.target)
		} else {
			targetPackage, _ = workspace.TargetPackage()
			b.target = targetPackage.Name
		}
	}

	// Fallback for no workspace
	if targetPackage.Name == "" {
		targetPackage, err = GetTargetPackage()
		if err != nil {
			log.Errorf("Unable to load package: %v\n", err)
			return subcommands.ExitFailure
		}
	}

	instructions := targetPackage.Manifest.Package

	if len(instructions.Artifacts) > 0 {
		if workspace.Path != "" {
			b.ArchiveWorkspace(targetPackage, instructions.Artifacts)
		}
	}
	if instructions.DockerArtifact.BaseImage != "" {
		b.PackageDockerImage(targetPackage, instructions.DockerArtifact)
	}

	return subcommands.ExitSuccess
}

//PackageDockerImage will create a push a container image matching the file format below
//
//artifacts:
// docker:
// 	 base_image: ubuntu
//   image: dispatcher
//   files:
//     - dispatcher:dispatcher
//   working_dir: /
//   exec: /dispatcher
func (b *PackageCmd) PackageDockerImage(targetPackage Package, artifact DockerArtifact) subcommands.ExitStatus {
	cd := narwhal.ContainerDefinition{
		Image:   artifact.BaseImage,
		Label:   artifact.Image,
		WorkDir: artifact.WorkingDir,
		Command: artifact.Exec,
	}

	file := ""
	localFile := ""
	// TODO: expand after talking John
	// UploadArchive for something like the API - How do we know which files?
	if len(artifact.Files) > 0 {
		files := strings.SplitN(artifact.Files[0], ":", 2)
		localFile = filepath.Join(targetPackage.Path(), files[0])
		file = files[0]
	}

	// Default to yourbase container registry.  User can pass in an empty string for local registry
	repository := targetPackage.Name
	if b.dockerRepository != "" {
		repository = fmt.Sprintf("%s/%s", strings.TrimRight(b.dockerRepository, "/"), targetPackage.Name)
	}

	err := narwhal.BuildImageWithFile(cd, repository, b.dockerTag, localFile, file, artifact.WorkingDir)
	if err != nil {
		log.Error(err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}

func (b *PackageCmd) ArchiveWorkspace(targetPackage Package, instructions []string) subcommands.ExitStatus {

	outputDir := filepath.Join(targetPackage.BuildRoot(), "output")
	MkdirAsNeeded(outputDir)
	archiveFile := fmt.Sprintf("%s-package.tar", targetPackage.Name)
	pkgFile := filepath.Join(outputDir, archiveFile)

	if PathExists(pkgFile) {
		os.Remove(pkgFile)
	}

	log.Infof("Generating package file %s...\n", pkgFile)

	tar := archiver.Tar{
		MkdirAll: true,
	}

	packageDir := targetPackage.Path()

	oldCwd, _ := os.Getwd()
	_ = os.Chdir(packageDir)
	err := tar.Archive(instructions, pkgFile)
	_ = os.Chdir(oldCwd)

	if err != nil {
		log.Errorf("Could not create archive: %v\n", err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}
