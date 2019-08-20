package cli

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"

	"github.com/johnewart/subcommands"

	. "github.com/yourbase/yb/packages"
	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
	. "github.com/yourbase/yb/workspace"
)

type CheckConfigCmd struct {
}

func (*CheckConfigCmd) Name() string     { return "checkconfig" }
func (*CheckConfigCmd) Synopsis() string { return "Check the config file syntax" }
func (*CheckConfigCmd) Usage() string {
	return `checkconfig`
}

func (b *CheckConfigCmd) SetFlags(f *flag.FlagSet) {
}

func (b *CheckConfigCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	var targetPackage Package

	if PathExists(MANIFEST_FILE) {
		currentPath, _ := filepath.Abs(".")
		_, pkgName := filepath.Split(currentPath)
		pkg, err := LoadPackage(pkgName, currentPath)
		if err != nil {
			fmt.Printf("Error loading package '%s': %v\n\nSee %s\n", pkgName, err, DOCS_URL)
			return subcommands.ExitFailure
		}
		targetPackage = pkg
	} else {

		workspace, err := LoadWorkspace()

		if err != nil {
			fmt.Printf("Could not find valid configuration: %v\n\nTry running in the package root dir or writing the YML config file (.yourbase.yml) if it is missing. See %s\n", err, DOCS_URL)
			return subcommands.ExitFailure
		}

		pkg, err := workspace.TargetPackage()
		if err != nil {
			fmt.Printf("Can't load workspace's target package: %v\n\nPackages under this Workspace may be missing a .yourbase.yml file or it's syntax is an invalid YML data. See %s\n", err, DOCS_URL)
			return subcommands.ExitFailure
		}

		targetPackage = pkg
	}

	fmt.Printf("Config syntax verified for package '%s', and it is successfully yourbased!\n", targetPackage.Name)

	return subcommands.ExitSuccess
}
