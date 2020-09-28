package buildpacks

import (
	"context"
	"fmt"
	"strings"

	"github.com/yourbase/yb/plumbing"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
	"zombiezen.com/go/log"
)

func tracer() trace.Tracer {
	return global.Tracer("github.com/yourbase/yb/buildpacks")
}

type buildToolSpec struct {
	tool            string
	version         string
	sharedCacheDir  string
	packageCacheDir string
	packageDir      string
}

// Install installs the buildpack given by spec.
func Install(ctx context.Context, pkgCacheDir string, pkgDir string, spec string) error {
	buildpackName, versionString, err := SplitToolSpec(spec)
	if err != nil {
		return fmt.Errorf("load build packs: %w", err)
	}

	sharedCacheDir := plumbing.ToolsDir()

	parsed := buildToolSpec{
		tool:            buildpackName,
		version:         versionString,
		sharedCacheDir:  sharedCacheDir,
		packageCacheDir: pkgCacheDir,
		packageDir:      pkgDir,
	}

	var bt interface {
		// install downloads the tool into the build environment.
		install(ctx context.Context) error

		// Setup sets environment variables in the current process (to be inherited by
		// subprocesses) from the build tool.
		//
		// TODO(light): This should return environment variables from Install rather
		// than modify global state.
		setup(ctx context.Context) error
	}
	log.Infof(ctx, "Configuring build tool: %s", spec)

	switch buildpackName {
	case "anaconda2":
		bt = newAnaconda2BuildTool(parsed)
	case "anaconda3":
		bt = newAnaconda3BuildTool(parsed)
	case "ant":
		bt = newAntBuildTool(parsed)
	case "r":
		bt = newRLangBuildTool(parsed)
	case "heroku":
		bt = newHerokuBuildTool(parsed)
	case "node":
		bt = newNodeBuildTool(parsed)
	case "yarn":
		bt = newYarnBuildTool(parsed)
	case "glide":
		bt = newGlideBuildTool(parsed)
	case "androidndk":
		bt = newAndroidNDKBuildTool(parsed)
	case "android":
		bt = newAndroidBuildTool(parsed)
	case "gradle":
		bt = newGradleBuildTool(parsed)
	case "flutter":
		bt = newFlutterBuildTool(parsed)
	case "dart":
		bt = newDartBuildTool(parsed)
	case "rust":
		bt = newRustBuildTool(parsed)
	case "java":
		bt = newJavaBuildTool(ctx, parsed)
	case "maven":
		bt = newMavenBuildTool(parsed)
	case "go":
		bt = newGolangBuildTool(parsed)
	case "python":
		bt = newPythonBuildTool(parsed)
	case "ruby":
		bt = newRubyBuildTool(parsed)
	case "homebrew":
		bt = newHomebrewBuildTool(parsed)
	case "protoc":
		bt = newProtocBuildTool(parsed)
	default:
		return fmt.Errorf("Unknown build tool: %s\n", spec)
	}

	// Install if needed
	_, installSpan := tracer().Start(ctx, spec+" [install]", trace.WithAttributes(
		label.String("buildpack", buildpackName),
		label.String("tool", spec),
	))
	err = bt.install(ctx)
	installSpan.End()
	if err != nil {
		installSpan.SetStatus(codes.Unknown, err.Error())
		return fmt.Errorf("Unable to install tool %s: %v", spec, err)
	}

	// Setup build tool (paths, env, etc)
	_, setupSpan := tracer().Start(ctx, spec+" [setup]", trace.WithAttributes(
		label.String("buildpack", buildpackName),
		label.String("tool", spec),
	))
	err = bt.setup(ctx)
	setupSpan.End()
	if err != nil {
		setupSpan.SetStatus(codes.Unknown, err.Error())
		return fmt.Errorf("Unable to setup tool %s: %v", spec, err)
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
