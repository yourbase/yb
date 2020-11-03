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

type runCmd struct {
	target      string
	noContainer bool
}

func newRunCmd() *cobra.Command {
	b := new(runCmd)
	c := &cobra.Command{
		Use:                   "run [options] COMMAND [ARG [...]]",
		Short:                 "Run an arbitrary command",
		Long:                  `Run a command in the target container.`,
		Args:                  cobra.MinimumNArgs(1),
		DisableFlagsInUseLine: true,
		SilenceErrors:         true,
		SilenceUsage:          true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return b.run(cmd.Context(), args)
		},
	}
	c.Flags().StringVarP(&b.target, "target", "t", "default", "The target to run the command in")
	c.Flags().BoolVar(&b.noContainer, "no-container", false, "Avoid using Docker if possible")
	return c
}

func (b *runCmd) run(ctx context.Context, args []string) error {
	dataDirs, err := ybdata.DirsFromEnv()
	if err != nil {
		return err
	}
	dockerClient, err := connectDockerClient(!b.noContainer)
	if err != nil {
		return err
	}
	pkg, err := GetTargetPackage()
	if err != nil {
		return err
	}
	targets, err := pkg.Manifest.BuildOrder(b.target)
	if err != nil {
		return err
	}
	dockerNetworkID, removeNetwork, err := newDockerNetwork(ctx, dockerClient)
	if err != nil {
		return err
	}
	defer removeNetwork()

	// Build dependencies before running command.
	err = doTargetList(ctx, pkg, targets[:len(targets)-1], &doOptions{
		dockerClient:    dockerClient,
		dockerNetworkID: dockerNetworkID,
		dataDirs:        dataDirs,
	})
	if err != nil {
		return err
	}

	// Run command.
	execTarget := targets[len(targets)-1]
	bio, err := newBiome(ctx, dockerClient, dataDirs, pkg.Path, execTarget.Name)
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
	phaseDeps, err := targetToPhaseDeps(execTarget)
	if err != nil {
		return err
	}
	execBiome, err := build.Setup(ctx, sys, phaseDeps)
	if err != nil {
		return err
	}
	defer func() {
		if err := execBiome.Close(); err != nil {
			log.Warnf(ctx, "Clean up environment: %v", err)
		}
	}()
	// TODO(ch2725): Run the command from the subdirectory the process is in.
	return execBiome.Run(ctx, &biome.Invocation{
		Argv:   args,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
}
