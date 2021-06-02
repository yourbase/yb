package buildpack

import (
	"context"
	"fmt"

	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
)

func installPython(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (biome.Environment, error) {
	envDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "miniforge-python", spec.Version())
	env, err := installMiniforge(ctx, sys)
	if err != nil {
		return biome.Environment{}, err
	}
	envBinDir := sys.Biome.JoinPath(envDir, "bin")
	// If environment already exists, return early.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, envDir); err == nil {
		env.PrependPath = append([]string{envBinDir}, env.PrependPath...)
		return env, nil
	}
	err = sys.Biome.Run(ctx, &biome.Invocation{
		Argv:   []string{"conda", "install", "-c", "anaconda", "setuptools"},
		Env:    env,
		Stdout: sys.Stdout,
		Stderr: sys.Stderr,
	})
	if err != nil {
		return biome.Environment{}, err
	}
	err = sys.Biome.Run(ctx, &biome.Invocation{
		Argv:   []string{"conda", "create", "--prefix", envDir, "python=" + spec.Version()},
		Env:    env,
		Stdout: sys.Stdout,
		Stderr: sys.Stderr,
	})
	if err != nil {
		return biome.Environment{}, fmt.Errorf("create environment: %w", err)
	}
	env.PrependPath = append([]string{envBinDir}, env.PrependPath...)
	return env, nil
}
