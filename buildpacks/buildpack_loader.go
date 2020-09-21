package buildpacks

import (
	"fmt"
	"strings"
	"time"

	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/types"
)

type BuildToolSpec struct {
	Tool            string
	Version         string
	SharedCacheDir  string
	PackageCacheDir string
	PackageDir      string
}

func LoadBuildPacks(dependencies []string, pkgCacheDir string, pkgDir string) ([]types.CommandTimer, error) {
	setupTimers := make([]types.CommandTimer, 0)

	for _, toolSpec := range dependencies {
		buildpackName, versionString, err := SplitToolSpec(toolSpec)
		if err != nil {
			return nil, fmt.Errorf("load build packs: %w", err)
		}

		sharedCacheDir := plumbing.ToolsDir()

		spec := BuildToolSpec{
			Tool:            buildpackName,
			Version:         versionString,
			SharedCacheDir:  sharedCacheDir,
			PackageCacheDir: pkgCacheDir,
			PackageDir:      pkgDir,
		}

		var bt types.BuildTool
		log.Infof("Configuring build tool: %s", toolSpec)

		switch buildpackName {
		case "anaconda2":
			bt = NewAnaconda2BuildTool(spec)
		case "anaconda3":
			bt = NewAnaconda3BuildTool(spec)
		case "ant":
			bt = NewAntBuildTool(spec)
		case "r":
			bt = NewRLangBuildTool(spec)
		case "heroku":
			bt = NewHerokuBuildTool(spec)
		case "node":
			bt = NewNodeBuildTool(spec)
		case "yarn":
			bt = NewYarnBuildTool(spec)
		case "glide":
			bt = NewGlideBuildTool(spec)
		case "androidndk":
			bt = NewAndroidNdkBuildTool(spec)
		case "android":
			bt = NewAndroidBuildTool(spec)
		case "gradle":
			bt = NewGradleBuildTool(spec)
		case "flutter":
			bt = NewFlutterBuildTool(spec)
		case "dart":
			bt = NewDartBuildTool(spec)
		case "rust":
			bt = NewRustBuildTool(spec)
		case "java":
			bt = NewJavaBuildTool(spec)
		case "maven":
			bt = NewMavenBuildTool(spec)
		case "go":
			bt = NewGolangBuildTool(spec)
		case "python":
			bt = NewPythonBuildTool(spec)
		case "ruby":
			bt = NewRubyBuildTool(spec)
		case "homebrew":
			bt = NewHomebrewBuildTool(spec)
		case "protoc":
			bt = NewProtocBuildTool(spec)
		default:
			return setupTimers, fmt.Errorf("Unknown build tool: %s\n", toolSpec)
		}

		// Install if needed
		startTime := time.Now()
		if err := bt.Install(); err != nil {
			return setupTimers, fmt.Errorf("Unable to install tool %s: %v", toolSpec, err)
		}
		endTime := time.Now()
		setupTimers = append(setupTimers, types.CommandTimer{
			Command:   fmt.Sprintf("%s [install]", toolSpec),
			StartTime: startTime,
			EndTime:   endTime,
		})

		// Setup build tool (paths, env, etc)
		startTime = time.Now()
		if err := bt.Setup(); err != nil {
			return setupTimers, fmt.Errorf("Unable to setup tool %s: %v", toolSpec, err)
		}
		endTime = time.Now()
		setupTimers = append(setupTimers, types.CommandTimer{
			Command:   fmt.Sprintf("%s [setup]", toolSpec),
			StartTime: startTime,
			EndTime:   endTime,
		})

	}

	return setupTimers, nil

}

func SplitToolSpec(dep string) (tool, version string, _ error) {
	parts := strings.SplitN(dep, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("malformed build pack definition: %s", dep)
	}
	tool = parts[0]
	version = parts[1]
	return
}
