package cli

import (
	"fmt"
	"path/filepath"

	. "github.com/yourbase/yb/packages"
	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
	. "github.com/yourbase/yb/workspace"
)

func GetTargetPackage() (Package, error) {
	var targetPackage Package

	// check if we're just a package

	if PathExists(MANIFEST_FILE) {
		currentPath, _ := filepath.Abs(".")
		_, pkgName := filepath.Split(currentPath)
		pkg, err := LoadPackage(pkgName, currentPath)
		if err != nil {
			return Package{}, fmt.Errorf("Error loading package '%s': %v\n\nSee %s\n", pkgName, err, DOCS_URL)
		}
		targetPackage = pkg
	} else {

		workspace, err := LoadWorkspace()

		if err != nil {

			return Package{}, fmt.Errorf("Could not find valid configuration: %v\n\nTry running in the package root dir or writing the YML config file (.yourbase.yml) if it is missing. See %s\n", err, DOCS_URL)
		}

		pkg, err := workspace.TargetPackage()
		if err != nil {
			return Package{}, fmt.Errorf("Can't load workspace's target package: %v\n\nPackages under this Workspace may be missing a .yourbase.yml file or it's syntax is an invalid YML data. See %s\n", err, DOCS_URL)
		}

		targetPackage = pkg
	}

	return targetPackage, nil
}
