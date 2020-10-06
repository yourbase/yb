package cli

import (
	"fmt"
	"path/filepath"

	"github.com/yourbase/yb/packages"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/types"
	"github.com/yourbase/yb/workspace"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
)

func tracer() trace.Tracer {
	return global.Tracer("github.com/yourbase/yb/cli")
}

func GetTargetPackageNamed(file string) (*packages.Package, error) {
	if file == "" {
		file = types.MANIFEST_FILE
	}

	// check if we're just a package

	if plumbing.PathExists(file) {
		currentPath, _ := filepath.Abs(".")
		_, pkgName := filepath.Split(currentPath)
		pkg, err := packages.LoadPackage(pkgName, currentPath)
		if err != nil {
			return nil, fmt.Errorf("Error loading package '%s': %w\n\nSee %s\n", pkgName, err, types.DOCS_URL)
		}
		return pkg, nil
	}

	workspace, err := workspace.LoadWorkspace()

	if err != nil {

		return nil, fmt.Errorf("Could not find valid configuration: %w\n\nTry running in the package root dir or writing the YML config file (%s) if it is missing. See %s", err, file, types.DOCS_URL)
	}

	pkg, err := workspace.TargetPackage()
	if err != nil {
		return nil, fmt.Errorf("Can't load workspace's target package: %w\n\nPackages under this Workspace may be missing a %s file or it's syntax is an invalid YML data. See %s", err, file, types.DOCS_URL)
	}

	return pkg, nil
}

func GetTargetPackage() (*packages.Package, error) {
	return GetTargetPackageNamed(types.MANIFEST_FILE)
}
