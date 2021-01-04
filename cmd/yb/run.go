package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/build"
	"github.com/yourbase/yb/internal/ybdata"
	"zombiezen.com/go/log"
)

type runCmd struct {
	env         []commandLineEnv
	netrcFiles  []string
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
	envFlagsVar(c.Flags(), &b.env)
	netrcFlagVar(c.Flags(), &b.netrcFiles)
	c.Flags().StringVarP(&b.target, "target", "t", yb.DefaultTarget, "The target to run the command in")
	c.Flags().BoolVar(&b.noContainer, "no-container", false, "Avoid using Docker if possible")
	c.RegisterFlagCompletionFunc("target", func(cc *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return autocompleteTargetName(toComplete)
	})
	return c
}

func (b *runCmd) run(ctx context.Context, args []string) error {
	dataDirs, err := ybdata.DirsFromEnv()
	if err != nil {
		return err
	}
	downloader := ybdata.NewDownloader(dataDirs.Downloads())
	baseEnv, err := envFromCommandLine(b.env)
	if err != nil {
		return err
	}
	dockerClient, err := connectDockerClient(!b.noContainer)
	if err != nil {
		return err
	}
	pkg, subdir, err := findPackage()
	if err != nil {
		return err
	}
	execTarget := pkg.Targets[b.target]
	if execTarget == nil {
		return fmt.Errorf("%s: no such target", b.target)
	}
	targets := yb.BuildOrder(execTarget)
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
		downloader:      downloader,
		baseEnv:         baseEnv,
		netrcFiles:      b.netrcFiles,
	})
	if err != nil {
		return err
	}

	// Run command.
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
	defer func() {
		if err := execBiome.Close(); err != nil {
			log.Warnf(ctx, "Clean up environment: %v", err)
		}
	}()
	subdirElems := strings.Split(subdir, string(filepath.Separator))
	return execBiome.Run(ctx, &biome.Invocation{
		Argv:        args,
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Dir:         execBiome.JoinPath(subdirElems...),
		Interactive: true,
	})
}
