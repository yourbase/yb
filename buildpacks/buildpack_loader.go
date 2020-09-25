package buildpacks

import (
	"context"
	"fmt"
	"strings"

	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/types"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
)

func tracer() trace.Tracer {
	return global.Tracer("github.com/yourbase/yb/buildpacks")
}

type BuildToolSpec struct {
	Tool            string
	Version         string
	SharedCacheDir  string
	PackageCacheDir string
	PackageDir      string
}

func LoadBuildPacks(ctx context.Context, dependencies []string, pkgCacheDir string, pkgDir string) error {
	for _, toolSpec := range dependencies {
		buildpackName, versionString, err := SplitToolSpec(toolSpec)
		if err != nil {
			return fmt.Errorf("load build packs: %w", err)
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
			return fmt.Errorf("Unknown build tool: %s\n", toolSpec)
		}

		// Install if needed
		_, installSpan := tracer().Start(ctx, toolSpec+" [install]", trace.WithAttributes(
			label.String("buildpack", buildpackName),
			label.String("tool", toolSpec),
		))
		err = bt.Install()
		installSpan.End()
		if err != nil {
			installSpan.SetStatus(codes.Unknown, err.Error())
			return fmt.Errorf("Unable to install tool %s: %v", toolSpec, err)
		}

		// Setup build tool (paths, env, etc)
		_, setupSpan := tracer().Start(ctx, toolSpec+" [setup]", trace.WithAttributes(
			label.String("buildpack", buildpackName),
			label.String("tool", toolSpec),
		))
		err = bt.Setup()
		setupSpan.End()
		if err != nil {
			setupSpan.SetStatus(codes.Unknown, err.Error())
			return fmt.Errorf("Unable to setup tool %s: %v", toolSpec, err)
		}
	}

	return nil
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
