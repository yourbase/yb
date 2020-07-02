package buildpacks

import (
	"context"
	"strings"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
	. "github.com/yourbase/yb/types"
)

func installPlatformDependencies(ctx context.Context, bt BuildTool) error {
	var spec BuildToolSpec

	// TODO reorg types to enable a generic way to treat this
	if rbTool, ok := bt.(RubyBuildTool); ok {
		spec = rbTool.spec
	} else if pyTool, ok := bt.(PythonBuildTool); ok {
		spec = pyTool.spec
	} else if hbTool, ok := bt.(HomebrewBuildTool); ok {
		spec = hbTool.spec
	}

	t := spec.InstallTarget

	switch t.OS() {
	case runtime.Darwin:
		if strings.HasPrefix(t.OSVersion(ctx), "18.") {
			// Need to install the headers on Mojave
			// Only needed on MetalTarget, when running on Mac OS X
			if _, ok := t.(*runtime.MetalTarget); ok && !t.PathExists(ctx, "/usr/include/zlib.h") {
				installCmd := "sudo -S installer -pkg /Library/Developer/CommandLineTools/Packages/macOS_SDK_headers_for_macOS_10.14.pkg -target /"
				log.Info("Going to run: ", installCmd)
				p := runtime.Process{
					Command:   installCmd,
					Directory: "/",
				}
				if err := t.Run(ctx, p); err != nil {
					log.Errorf("Unable to install buildpack dependency: %v", err)
					return err
				}
			}
		}
	}

	return nil
}
