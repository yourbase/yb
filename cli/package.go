package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/johnewart/archiver"
	"github.com/johnewart/subcommands"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/workspace"
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
func (b *PackageCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	return subcommands.ExitFailure
}

func (b *PackageCmd) ArchiveWorkspace() subcommands.ExitStatus {
	workspace, err := workspace.LoadWorkspace()

	if err != nil {
		return subcommands.ExitFailure
	}

	targetPackage, _ := workspace.TargetPackage()

	if b.target != "" {
		targetPackage, _ = workspace.PackageByName(b.target)
	}

	instructions := targetPackage.Manifest

	buildDir := workspace.BuildRoot()
	outputDir := filepath.Join(buildDir, "output")
	// TODO(light): This is being removed in https://github.com/yourbase/yb/pull/182
	os.MkdirAll(outputDir, 0777)
	archiveFile := fmt.Sprintf("%s-package.tar", targetPackage.Name)
	pkgFile := filepath.Join(outputDir, archiveFile)

	if plumbing.PathExists(pkgFile) {
		os.Remove(pkgFile)
	}

	log.Infof("Generating package file %s...\n", pkgFile)

	tar := archiver.Tar{
		MkdirAll: true,
	}

	packageDir := targetPackage.Path

	oldCwd, _ := os.Getwd()
	_ = os.Chdir(packageDir)
	err = tar.Archive(instructions.Package.Artifacts, pkgFile)
	_ = os.Chdir(oldCwd)

	if err != nil {
		log.Errorf("Could not create archive: %v\n", err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}
