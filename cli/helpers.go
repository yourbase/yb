package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/yourbase/yb/packages"
	"github.com/yourbase/yb/types"
)

func GetTargetPackageNamed(file string) (*packages.Package, error) {
	if _, err := os.Stat(file); err != nil {
		return nil, fmt.Errorf("could not find configuration: %w\n\nTry running in the package root dir or writing the YML config file (%s) if it is missing. See %s", err, file, types.DOCS_URL)
	}
	currentPath, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not find configuration: %w", err)
	}
	pkgName := filepath.Base(currentPath)
	pkg, err := packages.LoadPackage(pkgName, currentPath)
	if err != nil {
		return nil, fmt.Errorf("loading package %s: %w\n\nSee %s\n", pkgName, err, types.DOCS_URL)
	}
	return pkg, nil
}

func GetTargetPackage() (*packages.Package, error) {
	return GetTargetPackageNamed(types.MANIFEST_FILE)
}
