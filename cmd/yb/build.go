package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	"github.com/matishsiao/goInfo"
	"github.com/spf13/cobra"
	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/build"
	"github.com/yourbase/yb/internal/ybdata"
	"github.com/yourbase/yb/internal/ybtrace"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/codes"
	exporttrace "go.opentelemetry.io/otel/sdk/export/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"zombiezen.com/go/log"
)

const TIME_FORMAT = "15:04:05 MST"

type buildCmd struct {
	env              []commandLineEnv
	netrcFiles       []string
	execPrefix       string
	noContainer      bool
	dependenciesOnly bool
}

func newBuildCmd() *cobra.Command {
	b := new(buildCmd)
	c := &cobra.Command{
		Use:   "build [options] [TARGET]",
		Short: "Build a target",
		Long: `Builds a target in the current package. If no argument is given, ` +
			`uses the target named "` + yb.DefaultTarget + `", if there is one.`,
		Args:                  cobra.MaximumNArgs(1),
		DisableFlagsInUseLine: true,
		SilenceErrors:         true,
		SilenceUsage:          true,
		RunE: func(cmd *cobra.Command, args []string) error {
			target := yb.DefaultTarget
			if len(args) > 0 {
				target = args[0]
			}
			return b.run(cmd.Context(), target)
		},
		ValidArgsFunction: func(cc *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return autocompleteTargetName(toComplete)
		},
	}
	envFlagsVar(c.Flags(), &b.env)
	netrcFlagVar(c.Flags(), &b.netrcFiles)
	c.Flags().BoolVar(&b.noContainer, "no-container", false, "Avoid using Docker if possible")
	c.Flags().BoolVar(&b.dependenciesOnly, "deps-only", false, "Install only dependencies, don't do anything else")
	c.Flags().StringVar(&b.execPrefix, "exec-prefix", "", "Add a prefix to all executed commands (useful for timing or wrapping things)")
	return c
}

func (b *buildCmd) run(ctx context.Context, buildTargetName string) error {
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
	dockerClient, err := connectDockerClient(!b.noContainer)
	if err != nil {
		return err
	}

	startTime := time.Now()
	ctx, span := ybtrace.Start(ctx, "Build", trace.WithNewRoot())
	defer span.End()

	if insideTheMatrix() {
		startSection("BUILD")
	} else {
		startSection("BUILD HOST")
	}
	gi := goInfo.GetInfo()
	gi.VarDump()

	startSection("BUILD PACKAGE SETUP")
	log.Infof(ctx, "Build started at %s", startTime.Format(TIME_FORMAT))

	targetPackage, _, err := findPackage()
	if err != nil {
		return err
	}
	desired := targetPackage.Targets[buildTargetName]
	if desired == nil {
		return fmt.Errorf("%s: no such target (found: %s)", buildTargetName, strings.Join(listTargetNames(targetPackage.Targets), ", "))
	}
	buildTargets := yb.BuildOrder(desired)

	// Do the build!
	startSection("BUILD")
	log.Debugf(ctx, "Building package %s in %s...", targetPackage.Name, targetPackage.Path)

	buildError := doTargetList(ctx, targetPackage, buildTargets, &doOptions{
		dockerClient: dockerClient,
		dataDirs:     dataDirs,
		downloader:   downloader,
		execPrefix:   execPrefix,
		setupOnly:    b.dependenciesOnly,
		baseEnv:      baseEnv,
		netrcFiles:   b.netrcFiles,
	})
	if buildError != nil {
		span.SetStatus(codes.Unknown, buildError.Error())
	}
	span.End()
	endTime := time.Now()
	buildTime := endTime.Sub(startTime)

	log.Infof(ctx, "")
	log.Infof(ctx, "Build finished at %s, taking %s", endTime.Format(TIME_FORMAT), buildTime)
	log.Infof(ctx, "")

	log.Infof(ctx, "%s", buildTraces.dump())

	if buildError != nil {
		subSection("BUILD FAILED")
		return buildError
	}

	subSection("BUILD SUCCEEDED")
	return nil
}

type doOptions struct {
	dataDirs        *ybdata.Dirs
	downloader      *ybdata.Downloader
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
	if opts.dockerClient != nil && opts.dockerNetworkID == "" {
		opts2 := new(doOptions)
		*opts2 = *opts
		var cleanup func()
		var err error
		opts2.dockerNetworkID, cleanup, err = newDockerNetwork(ctx, opts.dockerClient)
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
	biomeOpts := newBiomeOptions{
		packageDir:      pkg.Path,
		target:          target.Name,
		dataDirs:        opts.dataDirs,
		downloader:      opts.downloader,
		baseEnv:         opts.baseEnv,
		netrcFiles:      opts.netrcFiles,
		dockerClient:    opts.dockerClient,
		targetContainer: target.Container,
		dockerNetworkID: opts.dockerNetworkID,
	}
	if target.HostOnly {
		biomeOpts = biomeOpts.disableDocker()
	}
	bio, err := newBiome(ctx, biomeOpts)
	if err != nil {
		return fmt.Errorf("target %s: %w", target.Name, err)
	}
	defer func() {
		if err := bio.Close(); err != nil {
			log.Warnf(ctx, "Clean up environment: %v", err)
		}
	}()
	sys := build.Sys{
		Biome:           bio,
		Downloader:      opts.downloader,
		DockerClient:    opts.dockerClient,
		DockerNetworkID: opts.dockerNetworkID,
		Stdout:          os.Stdout,
		Stderr:          os.Stderr,
	}
	execBiome, err := build.Setup(ctx, sys, target)
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

	subSection(fmt.Sprintf("Build target: %s", target.Name))
	log.Infof(ctx, "Executing build steps...")
	return build.Execute(ctx, sys, target)
}

// A traceSink records spans in memory. The zero value is an empty sink.
type traceSink struct {
	mu        sync.Mutex
	rootSpans []*exporttrace.SpanData
	children  map[trace.SpanID][]*exporttrace.SpanData
}

// ExportSpan saves the trace span. It is safe to be called concurrently.
func (sink *traceSink) ExportSpan(_ context.Context, span *exporttrace.SpanData) {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if !span.ParentSpanID.IsValid() {
		sink.rootSpans = append(sink.rootSpans, span)
		return
	}
	if sink.children == nil {
		sink.children = make(map[trace.SpanID][]*exporttrace.SpanData)
	}
	sink.children[span.ParentSpanID] = append(sink.children[span.ParentSpanID], span)
}

const (
	traceDumpStartWidth   = 14
	traceDumpEndWidth     = 14
	traceDumpElapsedWidth = 14
)

// dump formats the recorded traces as a hierarchial table of spans in the order
// received. It is safe to call concurrently, including with ExportSpan.
func (sink *traceSink) dump() string {
	sb := new(strings.Builder)
	fmt.Fprintf(sb, "%-*s %-*s %-*s\n",
		traceDumpStartWidth, "Start",
		traceDumpEndWidth, "End",
		traceDumpElapsedWidth, "Elapsed",
	)
	sink.mu.Lock()
	sink.dumpLocked(sb, trace.SpanID{}, 0)
	sink.mu.Unlock()
	return sb.String()
}

func (sink *traceSink) dumpLocked(sb *strings.Builder, parent trace.SpanID, depth int) {
	const indent = "  "
	list := sink.rootSpans
	if parent.IsValid() {
		list = sink.children[parent]
	}
	if depth >= 3 {
		if len(list) > 0 {
			writeSpaces(sb, traceDumpStartWidth+traceDumpEndWidth+traceDumpElapsedWidth+3)
			for i := 0; i < depth; i++ {
				sb.WriteString(indent)
			}
			sb.WriteString("...\n")
		}
		return
	}
	for _, span := range list {
		elapsed := span.EndTime.Sub(span.StartTime)
		fmt.Fprintf(sb, "%-*s %-*s %*.3fs %s\n",
			traceDumpStartWidth, span.StartTime.Format(TIME_FORMAT),
			traceDumpEndWidth, span.EndTime.Format(TIME_FORMAT),
			traceDumpElapsedWidth-1, elapsed.Seconds(),
			strings.Repeat(indent, depth)+span.Name,
		)
		sink.dumpLocked(sb, span.SpanContext.SpanID, depth+1)
	}
}

// Because, why not?
// Based on https://github.com/sindresorhus/is-docker/blob/master/index.js and https://github.com/moby/moby/issues/18355
// Discussion is not settled yet: https://stackoverflow.com/questions/23513045/how-to-check-if-a-process-is-running-inside-docker-container#25518538
func insideTheMatrix() bool {
	hasDockerEnv := pathExists("/.dockerenv")
	hasDockerCGroup := false
	dockerCGroupPath := "/proc/self/cgroup"
	if pathExists(dockerCGroupPath) {
		contents, _ := ioutil.ReadFile(dockerCGroupPath)
		hasDockerCGroup = strings.Count(string(contents), "docker") > 0
	}
	return hasDockerEnv || hasDockerCGroup
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return !os.IsNotExist(err)
}

func startSection(name string) {
	fmt.Printf(" === %s ===\n", name)
}

func subSection(name string) {
	fmt.Printf(" -- %s -- \n", name)
}

func writeSpaces(w io.ByteWriter, n int) {
	for i := 0; i < n; i++ {
		w.WriteByte(' ')
	}
}
