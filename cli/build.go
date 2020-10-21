package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	"github.com/johnewart/subcommands"
	"github.com/matishsiao/goInfo"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/build"
	"github.com/yourbase/yb/internal/ybdata"
	"github.com/yourbase/yb/internal/ybtrace"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/types"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/codes"
	exporttrace "go.opentelemetry.io/otel/sdk/export/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"zombiezen.com/go/log"
)

const TIME_FORMAT = "15:04:05 MST"

type BuildCmd struct {
	execPrefix       string
	noContainer      bool
	dependenciesOnly bool
}

func (*BuildCmd) Name() string     { return "build" }
func (*BuildCmd) Synopsis() string { return "Build the workspace" }
func (*BuildCmd) Usage() string {
	return `Usage: build [OPTIONS] [TARGET]
Build the project in the current directory. Defaults to the "default" target.
`
}

func (b *BuildCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&b.noContainer, "no-container", false, "Avoid using Docker if possible")
	f.BoolVar(&b.dependenciesOnly, "deps-only", false, "Install only dependencies, don't do anything else")
	f.StringVar(&b.execPrefix, "exec-prefix", "", "Add a prefix to all executed commands (useful for timing or wrapping things)")
}

func (b *BuildCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if f.NArg() > 1 {
		log.Errorf(ctx, "usage: yb build takes at most one target")
		return subcommands.ExitUsageError
	}

	// Set up trace sink.
	buildTraces := new(traceSink)
	tp, err := sdktrace.NewProvider(sdktrace.WithSyncer(buildTraces))
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	global.SetTraceProvider(tp)

	// Obtain global dependencies.
	dataDirs, err := ybdata.DirsFromEnv()
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	execPrefix, err := shlex.Split(b.execPrefix)
	if err != nil {
		log.Errorf(ctx, "Parse --exec-prefix: %v", err)
		return subcommands.ExitFailure
	}
	dockerClient, err := connectDockerClient(!b.noContainer)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}

	startTime := time.Now()
	ctx, span := ybtrace.Start(ctx, "Build", trace.WithNewRoot())
	defer span.End()

	if plumbing.InsideTheMatrix() {
		startSection("BUILD CONTAINER")
	} else {
		startSection("BUILD HOST")
	}
	gi := goInfo.GetInfo()
	gi.VarDump()

	startSection("BUILD PACKAGE SETUP")
	log.Infof(ctx, "Build started at %s", startTime.Format(TIME_FORMAT))

	targetPackage, err := GetTargetPackage()
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}

	// Determine targets to build.
	buildTargetName := "default"
	if f.NArg() > 0 {
		buildTargetName = f.Arg(0)
	}
	buildTargets, err := targetPackage.Manifest.BuildOrder(buildTargetName)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		log.Infof(ctx, "Valid build targets: %s", strings.Join(targetPackage.Manifest.BuildTargetList(), ", "))
		return subcommands.ExitFailure
	}

	// Do the build!
	startSection("BUILD")
	log.Debugf(ctx, "Building package %s in %s...", targetPackage.Name, targetPackage.Path)
	log.Debugf(ctx, "Checksum of dependencies: %s", targetPackage.Manifest.BuildDependenciesChecksum())

	buildError := doTargetList(ctx, targetPackage, buildTargets, &doOptions{
		dockerClient: dockerClient,
		dataDirs:     dataDirs,
		execPrefix:   execPrefix,
		setupOnly:    b.dependenciesOnly,
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
		log.Errorf(ctx, "Build terminated with the following error: %v", buildError)
		return subcommands.ExitFailure
	}

	subSection("BUILD SUCCEEDED")
	return subcommands.ExitSuccess
}

type doOptions struct {
	dataDirs        *ybdata.Dirs
	dockerClient    *docker.Client
	dockerNetworkID string
	execPrefix      []string
	setupOnly       bool
}

func doTargetList(ctx context.Context, pkg *types.Package, targets []*types.BuildTarget, opts *doOptions) error {
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

func doTarget(ctx context.Context, pkg *types.Package, target *types.BuildTarget, opts *doOptions) error {
	bio, err := newBiome(ctx, opts.dockerClient, opts.dataDirs, pkg.Path, target.Name)
	if err != nil {
		return fmt.Errorf("target %s: %w", target.Name, err)
	}
	sys := build.Sys{
		Biome:           bio,
		DataDirs:        opts.dataDirs,
		HTTPClient:      http.DefaultClient,
		DockerClient:    opts.dockerClient,
		DockerNetworkID: opts.dockerNetworkID,
		Stdout:          os.Stdout,
		Stderr:          os.Stderr,
	}
	phaseDeps, err := targetToPhaseDeps(target)
	if err != nil {
		return err
	}
	execBiome, err := build.Setup(ctx, sys, phaseDeps)
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
	err = build.Execute(ctx, sys, targetToPhase(target))
	if err != nil {
		return err
	}
	return nil
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
