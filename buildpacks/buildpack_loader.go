package buildpacks

import (
	"fmt"
	"strings"
	"time"

	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
)

type BuildToolSpec struct {
	Tool            string
	Version         string
	SharedCacheDir  string
	PackageCacheDir string
	PackageDir      string
}

func LoadBuildPacks(dependencies []string, pkgCacheDir string, pkgDir string) ([]CommandTimer, error) {
	setupTimers := make([]CommandTimer, 0)

	for _, toolSpec := range dependencies {

		parts := strings.Split(toolSpec, ":")
		buildpackName := parts[0]
		versionString := ""

		if len(parts) > 1 {
			versionString = parts[1]
		}

		sharedCacheDir := ToolsDir()

		spec := BuildToolSpec{
			Tool:            buildpackName,
			Version:         versionString,
			SharedCacheDir:  sharedCacheDir,
			PackageCacheDir: pkgCacheDir,
			PackageDir:      pkgDir,
		}

		var bt BuildTool
		fmt.Printf("Configuring build tool: %s\n", toolSpec)

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
		setupTimers = append(setupTimers, CommandTimer{
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
		setupTimers = append(setupTimers, CommandTimer{
			Command:   fmt.Sprintf("%s [setup]", toolSpec),
			StartTime: startTime,
			EndTime:   endTime,
		})

	}

	return setupTimers, nil

}
