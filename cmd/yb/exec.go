package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/build"
	"github.com/yourbase/yb/internal/ybdata"
	"zombiezen.com/go/log"
)

type execCmd struct {
	execEnvName string
	env         []commandLineEnv
	netrcFiles  []string
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
	netrcFlagVar(c.Flags(), &b.netrcFiles)
	// TODO(light): Use a less confusing name for this flag when it is using targets.
	c.Flags().StringVar(&b.execEnvName, "environment", yb.DefaultExecEnvironment, "Environment to run as")
	return c
}

func (b *execCmd) run(ctx context.Context) error {
	dataDirs, err := ybdata.DirsFromEnv()
	if err != nil {
		return err
	}
	downloader := ybdata.NewDownloader(dataDirs.Downloads())
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
	pkg, _, err := findPackage()
	if err != nil {
		return err
	}
	execTarget := pkg.ExecEnvironments[b.execEnvName]
	if execTarget == nil {
		return fmt.Errorf("exec %s: no such environment", b.execEnvName)
	}
	biomeOpts := newBiomeOptions{
		packageDir:      pkg.Path,
		target:          execTarget.Name,
		dataDirs:        dataDirs,
		downloader:      downloader,
		baseEnv:         baseEnv,
		netrcFiles:      b.netrcFiles,
		dockerClient:    dockerClient,
		targetContainer: execTarget.Container,
		dockerNetworkID: dockerNetworkID,
	}
	if execTarget.HostOnly {
		biomeOpts = biomeOpts.disableDocker()
	}
	bio, err := newBiome(ctx, biomeOpts)
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
		Downloader:      downloader,
		DockerClient:    dockerClient,
		DockerNetworkID: dockerNetworkID,
		Stdout:          os.Stdout,
		Stderr:          os.Stderr,
	}
	execBiome, err := build.Setup(ctx, sys, execTarget)
	if err != nil {
		return err
	}
	sys.Biome = execBiome
	defer func() {
		if err := execBiome.Close(); err != nil {
			log.Errorf(ctx, "Clean up environment %s: %v", b.execEnvName, err)
		}
	}()
	return build.Execute(ctx, sys, execTarget)
}
