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
	env        []commandLineEnv
	netrcFiles []string
	target     string
	mode       executionMode
}

func newRunCmd() *cobra.Command {
	b := new(runCmd)
	c := &cobra.Command{
		Use:   "run [options] COMMAND [ARG [...]]",
		Short: "Run an arbitrary command",
		Long: `Run a command in the target build environment.` +
			"\n\n" +
			`yb run will search for the .yourbase.yml file in the current directory ` +
			`and its parent directories. However, the command given in the command ` +
			`line will be run in the current working directory.`,
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
	executionModeVar(c.Flags(), &b.mode)
	c.Flags().StringVarP(&b.target, "target", "t", yb.DefaultTarget, "The target to run the command in!")
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
	dockerClient, err := connectDockerClient(b.mode)
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
	showDockerWarningsIfNeeded(ctx, b.mode, targets)
	dockerNetworkID, removeNetwork, err := newDockerNetwork(ctx, dockerClient, b.mode, targets)
	if err != nil {
		return err
	}
	defer removeNetwork()

	// Build dependencies before running command.
	err = doTargetList(ctx, pkg, targets[:len(targets)-1], &doOptions{
		output:          os.Stderr,
		executionMode:   b.mode,
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
	announceTarget(os.Stderr, execTarget.Name)
	bio, err := newBiome(ctx, execTarget, newBiomeOptions{
		executionMode:   b.mode,
		packageDir:      pkg.Path,
		dataDirs:        dataDirs,
		downloader:      downloader,
		baseEnv:         baseEnv,
		netrcFiles:      b.netrcFiles,
		dockerClient:    dockerClient,
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
		Downloader:      downloader,
		DockerClient:    dockerClient,
		DockerNetworkID: dockerNetworkID,
		Stdout:          os.Stderr,
		Stderr:          os.Stderr,
	}
	execBiome, err := build.Setup(withLogPrefix(ctx, execTarget.Name+setupLogPrefix), sys, execTarget)
	if err != nil {
		return err
	}
	defer func() {
		if err := execBiome.Close(); err != nil {
			log.Warnf(ctx, "Clean up environment: %v", err)
		}
	}()
	subdirElems := strings.Split(subdir, string(filepath.Separator))
	announceCommand(os.Stderr)(strings.Join(args, " "))
	return execBiome.Run(ctx, &biome.Invocation{
		Argv:        args,
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Dir:         execBiome.JoinPath(subdirElems...),
		Interactive: true,
	})
}
