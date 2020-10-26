package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/build"
	"github.com/yourbase/yb/internal/ybdata"
	"zombiezen.com/go/log"
)

const defaultExecEnvironment = "default"

type execCmd struct {
	environment string
}

func newExecCmd() *cobra.Command {
	b := new(execCmd)
	c := &cobra.Command{
		Use:           "exec",
		Short:         "Run the package",
		Long:          `Run the package using the instructions in the .yourbase.yml exec block.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return b.run(cmd.Context())
		},
	}
	c.Flags().StringVarP(&b.environment, "environment", "e", defaultExecEnvironment, "Environment to run as")
	return c
}

func (b *execCmd) run(ctx context.Context) error {
	dataDirs, err := ybdata.DirsFromEnv()
	if err != nil {
		return err
	}
	const useDocker = true
	dockerClient, err := connectDockerClient(useDocker)
	if err != nil {
		return err
	}
	dockerNetworkID, removeNetwork, err := newDockerNetwork(ctx, dockerClient)
	if err != nil {
		return err
	}
	defer removeNetwork()
	pkg, err := GetTargetPackage()
	if err != nil {
		return err
	}
	bio, err := newBiome(ctx, dockerClient, pkg.Path)
	if err != nil {
		return err
	}
	sys := build.Sys{
		Biome:           bio,
		DockerClient:    dockerClient,
		DockerNetworkID: dockerNetworkID,
		Stdout:          os.Stdout,
		Stderr:          os.Stderr,
	}
	defaultEnv, err := biome.MapVars(pkg.Manifest.Exec.Environment[defaultExecEnvironment])
	if err != nil {
		return err
	}
	deps := &build.PhaseDeps{
		TargetName:          b.environment,
		Resources:           narwhalContainerMap(pkg.Manifest.Exec.Dependencies.Containers),
		EnvironmentTemplate: defaultEnv,
	}
	if b.environment != defaultExecEnvironment {
		overrideEnv, err := biome.MapVars(pkg.Manifest.Exec.Environment[b.environment])
		if err != nil {
			return err
		}
		for k, v := range overrideEnv {
			deps.EnvironmentTemplate[k] = v
		}
	}
	execBiome, err := build.Setup(ctx, sys, deps)
	if err != nil {
		return err
	}
	sys.Biome = execBiome
	defer func() {
		if err := execBiome.Close(); err != nil {
			log.Errorf(ctx, "Clean up environment %s: %v", b.environment, err)
		}
	}()
	// TODO(ch2744): Move this into build.Setup.
	if err := pkg.SetupRuntimeDependencies(ctx, dataDirs); err != nil {
		return err
	}

	return build.Execute(ctx, sys, &build.Phase{
		TargetName: b.environment,
		Commands:   pkg.Manifest.Exec.Commands,
	})
}
