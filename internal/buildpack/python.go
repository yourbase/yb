package buildpack

import (
	"context"
	"fmt"

	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/types"
)

func installPython(ctx context.Context, sys Sys, spec types.BuildpackSpec) (biome.Environment, error) {
	envDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "conda-python", spec.Version())
	env, err := installAnaconda(ctx, sys, 3, "4.8.3")
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
