package cli

import (
	"context"
	"flag"
	"fmt"
	"github.com/johnewart/archiver"
	"github.com/johnewart/subcommands"
	"os"
	"path/filepath"

	. "github.com/yourbase/yb/plumbing"
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
func (b *PackageCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	return subcommands.ExitFailure
}

func (b *PackageCmd) ArchiveWorkspace() subcommands.ExitStatus {
	workspace, err := LoadWorkspace()

	if err != nil {
		return subcommands.ExitFailure
	}

	targetPackage, _ := workspace.TargetPackage()

	if b.target != "" {
		targetPackage, _ = workspace.PackageByName(b.target)
	}

	instructions := targetPackage.Manifest

	/*	fmt.Printf("Setting up dependencies...\n")
		workspace.SetupBuildDependencies(*instructions)

		/*fmt.Printf("Setting environment variables...\n")
		for _, property := range instructions.Exec.Environment {
			s := strings.Split(property, "=")
			if len(s) == 2 {
				fmt.Printf("  %s\n", s[0])
				os.Setenv(s[0], s[1])
			}
		}*/

	buildDir := workspace.BuildRoot()
	outputDir := filepath.Join(buildDir, "output")
	MkdirAsNeeded(outputDir)
	archiveFile := fmt.Sprintf("%s-package.tar", targetPackage)
	pkgFile := filepath.Join(outputDir, archiveFile)

	if PathExists(pkgFile) {
		os.Remove(pkgFile)
	}

	fmt.Printf("Generating package file %s...\n", pkgFile)

	tar := archiver.Tar{
		MkdirAll: true,
	}

	packageDir := targetPackage.Path

	oldCwd, _ := os.Getwd()
	_ = os.Chdir(packageDir)
	err = tar.Archive(instructions.Package.Artifacts, pkgFile)
	_ = os.Chdir(oldCwd)

	if err != nil {
		fmt.Printf("Could not create archive: %v\n", err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}
