package workspace

import (
	"context"
	"fmt"
	"time"

	"github.com/yourbase/yb/buildpacks"
	"github.com/yourbase/yb/runtime"

	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/types"
)

func LoadBuildPacks(ctx context.Context, installTarget runtime.Target, dependencies []string) ([]CommandTimer, error) {
	setupTimers := make([]CommandTimer, 0)

	for _, toolSpec := range dependencies {

		buildpackName, versionString, err := SplitToolSpec(toolSpec)
		if err != nil {
			return nil, fmt.Errorf("parsing a tool spec: %w", err)
		}

		spec := buildpacks.BuildToolSpec{
			Tool:          buildpackName,
			Version:       versionString,
			PackageDir:    installTarget.WorkDir(),
			InstallTarget: installTarget,
		}

		var bt BuildTool
		log.Infof("Configuring build tool %s in %s", toolSpec, installTarget)

		switch buildpackName {
		case "anaconda2":
			bt = buildpacks.NewAnaconda2BuildTool(spec)
		case "anaconda3":
			bt = buildpacks.NewAnaconda3BuildTool(spec)
		case "ant":
			bt = buildpacks.NewAntBuildTool(spec)
		case "r":
			bt = buildpacks.NewRLangBuildTool(spec)
		case "heroku":
			bt = buildpacks.NewHerokuBuildTool(spec)
		case "node":
			bt = buildpacks.NewNodeBuildTool(spec)
		case "yarn":
			bt = buildpacks.NewYarnBuildTool(spec)
		case "glide":
			bt = buildpacks.NewGlideBuildTool(spec)
		case "androidndk":
			bt = buildpacks.NewAndroidNdkBuildTool(spec)
		case "android":
			bt = buildpacks.NewAndroidBuildTool(spec)
		case "androidsdk":
			bt = buildpacks.NewAndroidBuildTool(spec)
		case "gradle":
			bt = buildpacks.NewGradleBuildTool(spec)
		case "flutter":
			bt = buildpacks.NewFlutterBuildTool(spec)
		case "dart":
			bt = buildpacks.NewDartBuildTool(spec)
		case "rust":
			bt = buildpacks.NewRustBuildTool(spec)
		case "java":
			bt = buildpacks.NewJavaBuildTool(spec)
		case "maven":
			bt = buildpacks.NewMavenBuildTool(spec)
		case "go":
			bt = buildpacks.NewGolangBuildTool(spec)
		case "python":
			bt = buildpacks.NewPythonBuildTool(spec)
		case "ruby":
			bt = buildpacks.NewRubyBuildTool(spec)
		case "homebrew":
			bt = buildpacks.NewHomebrewBuildTool(spec)
		case "protoc":
			bt = buildpacks.NewProtocBuildTool(spec)
		default:
			return setupTimers, fmt.Errorf("unknown build tool: %s\n", toolSpec)
		}

		// Install if needed
		startTime := time.Now()
		installedDir, err := bt.Install(ctx)
		if err != nil {
			return setupTimers, fmt.Errorf("installing tool %s: %v", toolSpec, err)
		}
		endTime := time.Now()
		setupTimers = append(setupTimers, CommandTimer{
			Command:   fmt.Sprintf("%s [install]", toolSpec),
			StartTime: startTime,
			EndTime:   endTime,
		})

		// Setup build tool (paths, env, etc)
		startTime = time.Now()
		if err := bt.Setup(ctx, installedDir); err != nil {
			return setupTimers, fmt.Errorf("setting up tool %s: %v", toolSpec, err)
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
