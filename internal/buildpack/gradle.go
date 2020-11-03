package buildpack

import (
	"context"
	"fmt"

	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log"
)

func installGradle(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (biome.Environment, error) {
	gradleDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "gradle", "gradle-"+spec.Version())
	env := biome.Environment{
		Vars: map[string]string{
			"GRADLE_USER_HOME": sys.Biome.JoinPath(sys.Biome.Dirs().Home, ".gradle"),
		},
		PrependPath: []string{
			sys.Biome.JoinPath(gradleDir, "bin"),
		},
	}

	// If directory already exists, then use it.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, gradleDir); err == nil {
		log.Infof(ctx, "Gradle v%s located in %s", spec.Version(), gradleDir)
		return env, nil
	}

	log.Infof(ctx, "Installing Gradle v%s in %s", spec.Version(), gradleDir)
	downloadURL := fmt.Sprintf("https://services.gradle.org/distributions/gradle-%s-bin.zip", spec.Version())
	if err := extract(ctx, sys, gradleDir, downloadURL, stripTopDirectory); err != nil {
		return biome.Environment{}, err
	}
	return env, nil
}
