package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/johnewart/archiver"
	"github.com/johnewart/subcommands"
	"os"

	"path/filepath"
)

type packageCmd struct {
	target string
}

func (*packageCmd) Name() string     { return "package" }
func (*packageCmd) Synopsis() string { return "Create a package artifact" }
func (*packageCmd) Usage() string {
	return `package [--target pkg]`
}

func (p *packageCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.target, "target", "", "Target package, if not the default")
}

/*
Executing the target involves:
1. Map source into the target container
2. Run any dependent components
3. Start target
*/
func (b *packageCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	workspace := LoadWorkspace()
	targetPackage := workspace.Target

	if b.target != "" {
		targetPackage = b.target
	}

	instructions, err := workspace.LoadPackageManifest(targetPackage)
	if err != nil {
		fmt.Printf("Error getting package manifest for %s: %v\n", targetPackage, err)
		return subcommands.ExitFailure
	}

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

	packageDir := workspace.PackagePath(targetPackage)

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
