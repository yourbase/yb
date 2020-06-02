package cli

import (
	"context"
	"flag"
	"fmt"
	"github.com/johnewart/archiver"
	"github.com/johnewart/subcommands"
	"os"
	"path/filepath"
	"strings"

	"github.com/yourbase/narwhal"
	. "github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/workspace"
)

type PackageCmd struct {
	target string
}

func (*PackageCmd) Name() string     { return "package" }
func (*PackageCmd) Synopsis() string { return "Create a package artifact" }
func (*PackageCmd) Usage() string {
	return `package [--target pkg]`
}

func (p *PackageCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.target, "target", "", "Target package, if not the default")
}

/*
Executing the target involves:
1. Map source into the target container
2. Run any dependent components
3. Start target
*/
func (b *PackageCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	cmdr := subcommands.NewCommander(f, "packages")

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

	cmdr.Register(&DockerArchiveCmd{targetPackage: targetPackage}, "")
	cmdr.Register(&TarArchiveCmd{targetPackage: targetPackage}, "")

	return (cmdr.Execute(ctx))
}

type DockerArchiveCmd struct {
	targetPackage  Package
	dockerRegistry string
	dockerTag      string
}

func (*DockerArchiveCmd) Name() string     { return "docker" }
func (*DockerArchiveCmd) Synopsis() string { return "Create a docker image" }
func (*DockerArchiveCmd) Usage() string {
	return ``
}

func (d *DockerArchiveCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&d.dockerRegistry, "registry", "docker-registry.yourbase.io", "Registry to upload image")
	f.StringVar(&d.dockerTag, "tag", "latest", "Container tag")
}

//Execute will create a container image matching the file format below
//
// package:
//   docker:
//     base_image: ubuntu
//     image: dispatcher
//     working_dir: /
//     exec: /dispatcher
//   artifacts:
//     - dispatcher
func (d *DockerArchiveCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	instructions := d.targetPackage.Manifest.Package
	if instructions.Docker.BaseImage == "" {
		log.Errorf("BaseImage is a required field in the configuration")
		return subcommands.ExitFailure
	}
	cd := narwhal.ContainerDefinition{
		Image:   instructions.Docker.BaseImage,
		WorkDir: instructions.Docker.WorkingDir,
		Command: instructions.Docker.Exec,
	}

	tmpArchiveFile := filepath.Join(d.targetPackage.Path(), fmt.Sprintf("%s-package.tar", d.targetPackage.Name))
	tar := archiver.Tar{MkdirAll: true}
	err := tar.Archive(instructions.Artifacts, tmpArchiveFile)
	if err != nil {
		log.Errorf("Could not create archive: %v\n", err)
		return subcommands.ExitFailure
	}
	defer os.Remove(tmpArchiveFile)

	// Default to yourbase container registry.  User can pass in an empty string for local registry
	registry := d.targetPackage.Name
	if d.dockerRegistry != "" {
		registry = fmt.Sprintf("%s/%s", strings.TrimRight(d.dockerRegistry, "/"), d.targetPackage.Name)
	}

	err = narwhal.BuildImageWithArchive(cd, registry, d.dockerTag, tmpArchiveFile, "tmp.tar", instructions.Docker.WorkingDir)
	if err != nil {
		log.Error(err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}

type TarArchiveCmd struct {
	targetPackage Package
	tar_file      string
}

func (*TarArchiveCmd) Name() string     { return "tar" }
func (*TarArchiveCmd) Synopsis() string { return "Create a tar file" }
func (*TarArchiveCmd) Usage() string {
	return ``
}

func (t *TarArchiveCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&t.tar_file, "tar_file", "", "tar file path and name")
}

func (t *TarArchiveCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	if t.tar_file == "" {
		outputDir := filepath.Join(t.targetPackage.BuildRoot(), "output")
		MkdirAsNeeded(outputDir)
		archiveFile := fmt.Sprintf("%s-package.tar", t.targetPackage.Name)
		t.tar_file = filepath.Join(outputDir, archiveFile)
	}

	if PathExists(t.tar_file) {
		os.Remove(t.tar_file)
	}

	log.Infof("Generating package file %s...\n", t.tar_file)

	tar := archiver.Tar{
		MkdirAll: true,
	}

	packageDir := t.targetPackage.Path()

	oldCwd, _ := os.Getwd()
	_ = os.Chdir(packageDir)
	err := tar.Archive(t.targetPackage.Manifest.Package.Artifacts, t.tar_file)
	_ = os.Chdir(oldCwd)

	if err != nil {
		log.Errorf("Could not create archive: %v\n", err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}
