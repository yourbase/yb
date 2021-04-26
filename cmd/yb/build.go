package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	"github.com/spf13/cobra"
	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/build"
	"github.com/yourbase/yb/internal/ybdata"
	"github.com/yourbase/yb/internal/ybtrace"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"zombiezen.com/go/log"
)

type buildCmd struct {
	targetNames      []string
	env              []commandLineEnv
	netrcFiles       []string
	execPrefix       string
	mode             executionMode
	dependenciesOnly bool
}

func newBuildCmd() *cobra.Command {
	b := new(buildCmd)
	c := &cobra.Command{
		Use:   "build [options] [TARGET [...]]",
		Short: "Build target(s)",
		Long: `Builds one or more targets in the current package. If no argument is given, ` +
			`uses the target named "` + yb.DefaultTarget + `", if there is one.` +
			"\n\n" +
			`yb build will search for the .yourbase.yml file in the current directory ` +
			`and its parent directories. The target's commands will be run in the ` +
			`directory the .yourbase.yml file appears in.`,
		Args:                  cobra.ArbitraryArgs,
		DisableFlagsInUseLine: true,
		SilenceErrors:         true,
		SilenceUsage:          true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				b.targetNames = []string{yb.DefaultTarget}
			} else {
				b.targetNames = args
			}
			return b.run(cmd.Context())
		},
		ValidArgsFunction: func(cc *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return autocompleteTargetName(toComplete)
		},
	}
	envFlagsVar(c.Flags(), &b.env)
	netrcFlagVar(c.Flags(), &b.netrcFiles)
	executionModeVar(c.Flags(), &b.mode)
	c.Flags().BoolVar(&b.dependenciesOnly, "deps-only", false, "Install only dependencies, don't do anything else")
	c.Flags().StringVar(&b.execPrefix, "exec-prefix", "", "Add a prefix to all executed commands (useful for timing or wrapping things)")
	return c
}

func (b *buildCmd) run(ctx context.Context) error {
	// Set up trace sink.
	buildTraces := new(traceSink)
	tp, err := sdktrace.NewProvider(sdktrace.WithSyncer(buildTraces))
	if err != nil {
		return err
	}
	global.SetTraceProvider(tp)

	// Obtain global dependencies.
	dataDirs, err := ybdata.DirsFromEnv()
	if err != nil {
		return err
	}
	downloader := ybdata.NewDownloader(dataDirs.Downloads())
	baseEnv, err := envFromCommandLine(b.env)
	if err != nil {
		return err
	}
	execPrefix, err := shlex.Split(b.execPrefix)
	if err != nil {
		return fmt.Errorf("parse --exec-prefix: %w", err)
	}
	dockerClient, err := connectDockerClient(b.mode)
	if err != nil {
		return err
	}

	startTime := time.Now()
	ctx, span := ybtrace.Start(ctx, "Build", trace.WithNewRoot())
	defer span.End()
	ctx = withStdoutLogs(ctx)

	log.Infof(ctx, "Build started at %s", startTime.Format(longTimeFormat))

	targetPackage, _, err := findPackage()
	if err != nil {
		return err
	}
	desired := make([]*yb.Target, 0, len(b.targetNames))
	for _, name := range b.targetNames {
		target := targetPackage.Targets[name]
		if target == nil {
			return fmt.Errorf("%s: no such target (found: %s)", name, strings.Join(listTargetNames(targetPackage.Targets), ", "))
		}
		desired = append(desired, target)
	}
	buildTargets := yb.BuildOrder(desired...)
	showDockerWarningsIfNeeded(ctx, b.mode, buildTargets)

	// Do the build!
	log.Debugf(ctx, "Building package %s in %s...", targetPackage.Name, targetPackage.Path)

	buildError := doTargetList(ctx, targetPackage, buildTargets, &doOptions{
		output:        os.Stdout,
		executionMode: b.mode,
		dockerClient:  dockerClient,
		dataDirs:      dataDirs,
		downloader:    downloader,
		execPrefix:    execPrefix,
		setupOnly:     b.dependenciesOnly,
		baseEnv:       baseEnv,
		netrcFiles:    b.netrcFiles,
	})
	if buildError != nil {
		span.SetStatus(codes.Unknown, buildError.Error())
		log.Errorf(ctx, "%v", buildError)
	}
	span.End()
	endTime := time.Now()
	buildTime := endTime.Sub(startTime)

	fmt.Printf("\nBuild finished at %s, taking %v\n\n", endTime.Format(longTimeFormat), buildTime.Truncate(time.Millisecond))
	fmt.Println(buildTraces.dump())

	style := termStylesFromEnv()
	if buildError != nil {
		fmt.Printf("%sBUILD FAILED%s ❌\n", style.buildResult(false), style.reset())
		return alreadyLoggedError{buildError}
	}

	fmt.Printf("%sBUILD PASSED%s ️✔️\n", style.buildResult(true), style.reset())
	return nil
}

type doOptions struct {
	output          io.Writer
	dataDirs        *ybdata.Dirs
	downloader      *ybdata.Downloader
	executionMode   executionMode
	dockerClient    *docker.Client
	dockerNetworkID string
	baseEnv         biome.Environment
	netrcFiles      []string
	execPrefix      []string
	setupOnly       bool
}

func doTargetList(ctx context.Context, pkg *yb.Package, targets []*yb.Target, opts *doOptions) error {
	if len(targets) == 0 {
		return nil
	}
	orderMsg := new(strings.Builder)
	orderMsg.WriteString("Going to build targets in the following order:")
	for _, target := range targets {
		fmt.Fprintf(orderMsg, "\n   - %s", target.Name)
	}
	log.Debugf(ctx, "%s", orderMsg)

	// Create a Docker network, if needed.
	if opts.dockerNetworkID == "" {
		opts2 := new(doOptions)
		*opts2 = *opts
		var cleanup func()
		var err error
		opts2.dockerNetworkID, cleanup, err = newDockerNetwork(ctx, opts.dockerClient, opts.executionMode, targets)
		if err != nil {
			return err
		}
		defer cleanup()
		opts = opts2
	}
	for _, target := range targets {
		err := doTarget(ctx, pkg, target, opts)
		if err != nil {
			return err
		}
	}
	return nil
}

func doTarget(ctx context.Context, pkg *yb.Package, target *yb.Target, opts *doOptions) error {
	style := termStylesFromEnv()
	fmt.Printf("\n🎯 %sTarget: %s%s\n", style.target(), target.Name, style.reset())

	ctx = withLogPrefix(ctx, target.Name)

	bio, err := newBiome(ctx, target, newBiomeOptions{
		packageDir:      pkg.Path,
		dataDirs:        opts.dataDirs,
		downloader:      opts.downloader,
		baseEnv:         opts.baseEnv,
		netrcFiles:      opts.netrcFiles,
		executionMode:   opts.executionMode,
		dockerClient:    opts.dockerClient,
		dockerNetworkID: opts.dockerNetworkID,
	})
	if err != nil {
		return fmt.Errorf("target %s: %w", target.Name, err)
	}
	defer func() {
		if err := bio.Close(); err != nil {
			log.Warnf(ctx, "Clean up environment: %v", err)
		}
	}()
	output := newLinePrefixWriter(opts.output, target.Name)
	sys := build.Sys{
		Biome:           bio,
		Downloader:      opts.downloader,
		DockerClient:    opts.dockerClient,
		DockerNetworkID: opts.dockerNetworkID,

		Stdout: output,
		Stderr: output,
	}
	execBiome, err := build.Setup(withLogPrefix(ctx, setupLogPrefix), sys, target)
	if err != nil {
		return err
	}
	sys.Biome = biome.ExecPrefix{
		Biome:       execBiome,
		PrependArgv: opts.execPrefix,
	}
	defer func() {
		if err := execBiome.Close(); err != nil {
			log.Errorf(ctx, "Clean up target %s: %v", target.Name, err)
		}
	}()
	if opts.setupOnly {
		return nil
	}

	return build.Execute(withStdoutLogs(ctx), sys, announceCommand, target)
}

func announceCommand(cmdString string) {
	style := termStylesFromEnv()
	fmt.Printf("%s> %s%s\n", style.command(), cmdString, style.reset())
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return !os.IsNotExist(err)
}
