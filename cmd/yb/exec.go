package main

import (
	"context"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/build"
	"github.com/yourbase/yb/internal/ybdata"
	"zombiezen.com/go/log"
)

const defaultExecEnvironment = "default"

type execCmd struct {
	execEnvName string
	env         []commandLineEnv
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
	envFlagsVar(c.Flags(), &b.env)
	// TODO(light): Use a less confusing name for this flag when it is using targets.
	c.Flags().StringVarP(&b.execEnvName, "environment", "", defaultExecEnvironment, "Environment to run as")
	return c
}

func (b *execCmd) run(ctx context.Context) error {
	dataDirs, err := ybdata.DirsFromEnv()
	if err != nil {
		return err
	}
	baseEnv, err := envFromCommandLine(b.env)
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
	bio, err := newBiome(ctx, newBiomeOptions{
		packageDir:      pkg.Path,
		target:          b.execEnvName,
		dataDirs:        dataDirs,
		baseEnv:         baseEnv,
		dockerClient:    dockerClient,
		targetContainer: pkg.Manifest.Exec.Container.ToNarwhal(),
		dockerNetworkID: dockerNetworkID,
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := bio.Close(); err != nil {
			log.Warnf(ctx, "Clean up environment: %v", err)
		}
	}()
	sys := build.Sys{
		Biome:           bio,
		DataDirs:        dataDirs,
		HTTPClient:      http.DefaultClient,
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
		TargetName:          b.execEnvName,
		Resources:           narwhalContainerMap(pkg.Manifest.Exec.Dependencies.Containers),
		EnvironmentTemplate: defaultEnv,
	}
	if b.execEnvName != defaultExecEnvironment {
		overrideEnv, err := biome.MapVars(pkg.Manifest.Exec.Environment[b.execEnvName])
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
			log.Errorf(ctx, "Clean up environment %s: %v", b.execEnvName, err)
		}
	}()

	return build.Execute(ctx, sys, &build.Phase{
		TargetName: b.execEnvName,
		Commands:   pkg.Manifest.Exec.Commands,
	})
}
