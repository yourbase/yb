package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/johnewart/subcommands"
	"github.com/matishsiao/goInfo"
	"github.com/yourbase/commons/xcontext"
	"github.com/yourbase/narwhal"
	"github.com/yourbase/yb/internal/ybdata"
	"github.com/yourbase/yb/internal/ybtrace"
	"github.com/yourbase/yb/packages"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/types"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
	exporttrace "go.opentelemetry.io/otel/sdk/export/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"zombiezen.com/go/log"
)

const TIME_FORMAT = "15:04:05 MST"

type BuildCmd struct {
	ExecPrefix       string
	NoContainer      bool
	NoSideContainer  bool
	DependenciesOnly bool
	ReuseContainers  bool
}

type BuildConfiguration struct {
	Target           *types.BuildTarget
	TargetDir        string
	ExecPrefix       string
	ForceNoContainer bool
	TargetPackage    *packages.Package
	BuildData        BuildData
	ReuseContainers  bool
}

func (*BuildCmd) Name() string     { return "build" }
func (*BuildCmd) Synopsis() string { return "Build the workspace" }
func (*BuildCmd) Usage() string {
	return `Usage: build [TARGET] [OPTIONS]
Build the project in the current directory. Defaults to the "default" target.
`
}

func (b *BuildCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&b.NoContainer, "no-container", false, "Bypass container even if specified")
	f.BoolVar(&b.DependenciesOnly, "deps-only", false, "Install only dependencies, don't do anything else")
	f.StringVar(&b.ExecPrefix, "exec-prefix", "", "Add a prefix to all executed commands (useful for timing or wrapping things)")
	f.BoolVar(&b.ReuseContainers, "reuse-containers", true, "Reuse the container for building")
	f.BoolVar(&b.NoSideContainer, "no-side-container", false, "Don't run/create any side container")
}

func (b *BuildCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if f.NArg() > 1 {
		log.Errorf(ctx, "usage: yb build takes at most one target")
		return subcommands.ExitUsageError
	}
	buildTraces := new(traceSink)
	tp, err := sdktrace.NewProvider(sdktrace.WithSyncer(buildTraces))
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	global.SetTraceProvider(tp)

	dataDirs, err := ybdata.DirsFromEnv()
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

	manifest := targetPackage.Manifest
	workDir, err := targetPackage.BuildRoot(dataDirs)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}

	// Determine build target
	buildTargetName := "default"

	if len(f.Args()) > 0 {
		buildTargetName = f.Args()[0]
	}

	// Named target, look for that and resolve it
	buildTargets, err := manifest.ResolveBuildTargets(buildTargetName)

	if err != nil {
		log.Errorf(ctx, "Could not compute build target '%s': %v", buildTargetName, err)
		log.Infof(ctx, "Valid build targets: %s", strings.Join(manifest.BuildTargetList(), ", "))
		return subcommands.ExitFailure
	}

	primaryTarget, err := manifest.BuildTarget(buildTargetName)
	if err != nil {
		log.Errorf(ctx, "Couldn't get primary build target '%s' specs: %v", buildTargetName, err)
		return subcommands.ExitFailure
	}

	buildData := NewBuildData()

	// Start resources.
	if !b.NoContainer && !b.NoSideContainer && len(primaryTarget.Dependencies.Containers) > 0 {
		dockerClient, err := docker.NewVersionedClient("unix:///var/run/docker.sock", "1.39")
		if err != nil {
			log.Errorf(ctx, "%v", err)
			return subcommands.ExitFailure
		}
		if err := narwhal.DockerClient().Ping(); err != nil {
			log.Errorf(ctx, "Couldn't connect to Docker daemon. Try installing Docker Desktop: https://hub.docker.com/search/?type=edition&offering=community")
			return subcommands.ExitFailure
		}
		contextID := targetPackage.Name + "-" + primaryTarget.Name
		sc, err := narwhal.NewServiceContextWithId(ctx, dockerClient, contextID, workDir)
		if err != nil {
			log.Errorf(ctx, "Couldn't create service context for dependencies: %v", err)
			return subcommands.ExitFailure
		}

		buildData.containers.serviceContext = sc

		log.Infof(ctx, "Starting %d containers with context id %s...", len(primaryTarget.Dependencies.Containers), contextID)
		if !b.ReuseContainers {
			log.Infof(ctx, "Not reusing containers, will teardown existing ones and clean up after ourselves")
			defer Cleanup(xcontext.IgnoreDeadline(ctx), buildData)
			if err := sc.TearDown(xcontext.IgnoreDeadline(ctx)); err != nil {
				log.Errorf(ctx, "Couldn't terminate existing containers: %v", err)
				// FAIL?
			}
		}

		for label, cd := range primaryTarget.Dependencies.Containers {
			c, err := sc.StartContainer(ctx, os.Stderr, cd.ToNarwhal())
			if err != nil {
				log.Errorf(ctx, "Couldn't start dependencies: %v", err)
				return subcommands.ExitFailure
			}
			buildData.containers.containers[label] = c
		}
	}

	// Expand environment variables. Depends on container IP addresses.
	exp, err := newConfigExpansion(ctx, buildData.containers, primaryTarget.Dependencies.Containers)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	for _, envString := range primaryTarget.Environment {
		parts := strings.SplitN(envString, "=", 2)
		if len(parts) != 2 {
			log.Warnf(ctx, "'%s' doesn't look like an environment variable", envString)
			continue
		}
		if err := buildData.SetEnv(exp, parts[0], parts[1]); err != nil {
			log.Errorf(ctx, "%v", err)
			return subcommands.ExitFailure
		}
	}

	// Do the build!
	targetDir := targetPackage.Path

	log.Infof(ctx, "Building package %s in %s...", targetPackage.Name, targetDir)
	log.Infof(ctx, "Checksum of dependencies: %s", manifest.BuildDependenciesChecksum())

	if len(primaryTarget.Dependencies.Containers) > 0 {
		startSection("CONTAINER SETUP")
		log.Infof(ctx, "")
		log.Infof(ctx, "")
		log.Infof(ctx, "Available side containers:")
		for label, c := range primaryTarget.Dependencies.Containers {
			ipv4, err := buildData.containers.IP(ctx, label)
			if err == nil {
				log.Infof(ctx, "  * %s (using %s) has IP address %s", label, c.ToNarwhal().ImageNameWithTag(), ipv4)
			} else {
				log.Warnf(ctx, "  * %s (using %s) has unknown IP address: %v", label, c.ToNarwhal().ImageNameWithTag(), err)
			}
		}
	}

	startSection("BUILD")

	config := BuildConfiguration{
		ExecPrefix:       b.ExecPrefix,
		TargetDir:        targetDir,
		ForceNoContainer: b.NoContainer,
		TargetPackage:    targetPackage,
		ReuseContainers:  b.ReuseContainers,
		BuildData:        buildData,
	}

	log.Infof(ctx, "Going to build targets in the following order: ")
	for _, target := range buildTargets {
		log.Infof(ctx, "   - %s", target.Name)
	}

	var buildError error
	for _, target := range buildTargets {
		targetCtx, targetSpan := ybtrace.Start(ctx, target.Name, trace.WithAttributes(
			label.String("target", target.Name),
		))
		buildError = targetPackage.SetupBuildDependencies(targetCtx, dataDirs, target)
		if buildError != nil {
			buildError = fmt.Errorf("setting up dependencies for target %s: %w", target.Name, buildError)
			targetSpan.SetStatus(codes.Internal, buildError.Error())
			targetSpan.End()
			break
		}

		if b.DependenciesOnly {
			continue
		}
		subSection(fmt.Sprintf("Build target: %s", target.Name))
		config.Target = target
		buildData.ExportEnvironmentPublicly(ctx)
		log.Infof(ctx, "Executing build steps...")
		buildError = RunCommands(targetCtx, config)
		if buildError != nil {
			buildError = fmt.Errorf("running commands for target %s: %w", target.Name, buildError)
			targetSpan.SetStatus(codes.Unknown, buildError.Error())
		}
		targetSpan.End()
		buildData.UnexportEnvironmentPublicly(ctx)
		if buildError != nil {
			break
		}
	}
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
	} else {
		subSection("BUILD SUCCEEDED")
	}

	if buildError != nil {
		return subcommands.ExitFailure
	}

	// No errors, :+1:
	return subcommands.ExitSuccess
}

func Cleanup(ctx context.Context, b BuildData) {
	if b.containers.serviceContext != nil {
		log.Infof(ctx, "Cleaning up containers...")
		if err := b.containers.serviceContext.TearDown(ctx); err != nil {
			log.Warnf(ctx, "Problem tearing down containers: %v", err)
		}
	}
}

func RunCommands(ctx context.Context, config BuildConfiguration) error {
	target := config.Target
	targetDir := config.TargetDir
	if target.Root != "" {
		log.Infof(ctx, "Build root is %s", target.Root)
		targetDir = filepath.Join(targetDir, target.Root)
	}

	for _, cmdString := range target.Commands {
		var stepError error

		_, cmdSpan := ybtrace.Start(ctx, cmdString, trace.WithAttributes(
			label.String("command", cmdString),
		))
		if strings.HasPrefix(cmdString, "cd ") {
			parts := strings.SplitN(cmdString, " ", 2)
			dir := filepath.Join(targetDir, parts[1])
			//log.Infof(ctx, "Chdir to %s", dir)
			//os.Chdir(dir)
			targetDir = dir
		} else {
			if len(config.ExecPrefix) > 0 {
				cmdString = fmt.Sprintf("%s %s", config.ExecPrefix, cmdString)
			}

			if err := plumbing.ExecToStdout(cmdString, targetDir); err != nil {
				log.Errorf(ctx, "Failed to run %s: %v", cmdString, err)
				stepError = err
				cmdSpan.SetStatus(codes.Unknown, err.Error())
			}
		}
		cmdSpan.End()

		// Make sure our goroutine gets this from stdout
		// TODO: There must be a better way...
		time.Sleep(10 * time.Millisecond)
		if stepError != nil {
			return stepError
		}
	}

	return nil
}

// configExpansion expands templated substitutions in parts of the
// configuration file. The object itself is passed into text/template,
// so its public fields are public API surface.
type configExpansion struct {
	// Containers holds the set of resources for the target.
	// The field name is public API surface and must not change.
	Containers containersExpansion
}

type containersExpansion struct {
	ips map[string]string
}

func newConfigExpansion(ctx context.Context, c containerData, containers map[string]*types.ContainerDefinition) (configExpansion, error) {
	exp := configExpansion{
		Containers: containersExpansion{
			ips: make(map[string]string),
		},
	}
	for label := range containers {
		ip, err := c.IP(ctx, label)
		if err != nil {
			return configExpansion{}, err
		}
		exp.Containers.ips[label] = ip
	}
	return exp, nil
}

func (exp configExpansion) expand(value string) (string, error) {
	return plumbing.TemplateToString(value, exp)
}

// IP returns the IP address of a particular container.
// The signature of this method is public API surface and must not change.
func (exp containersExpansion) IP(label string) (string, error) {
	ip := exp.ips[label]
	if ip == "" {
		return "", fmt.Errorf("find IP for %s: unknown container", label)
	}
	return ip, nil
}

// Build metadata; used for things like interpolation in environment variables
type containerData struct {
	serviceContext *narwhal.ServiceContext
	containers     map[string]*narwhal.Container
}

func (c containerData) IP(ctx context.Context, label string) (string, error) {
	if buildContainer := c.containers[label]; buildContainer != nil && c.serviceContext != nil {
		ip, err := narwhal.IPv4Address(ctx, c.serviceContext.DockerClient, buildContainer.Id)
		if err != nil {
			return "", fmt.Errorf("find IP for %s: %w", label, err)
		}
		return ip.String(), nil
	}

	// Look for environment variable (injected into containers)
	ip := os.Getenv("YB_CONTAINER_" + strings.ToUpper(label) + "_IP")
	if ip == "" {
		return "", fmt.Errorf("find IP for %s: unknown container", label)
	}
	return ip, nil
}

func (c containerData) Environment(ctx context.Context) map[string]string {
	result := make(map[string]string)
	if c.serviceContext != nil {
		for label, container := range c.containers {
			ipv4, err := narwhal.IPv4Address(ctx, c.serviceContext.DockerClient, container.Id)
			if err != nil {
				log.Warnf(ctx, "Obtaining address for %s container: %v", label, err)
				continue
			}
			key := fmt.Sprintf("YB_CONTAINER_%s_IP", strings.ToUpper(label))
			result[key] = ipv4.String()
		}
	}
	return result
}

type BuildData struct {
	containers  containerData
	environment map[string]string
	originalEnv map[string]string
}

func NewBuildData() BuildData {
	osEnv := os.Environ()
	originalEnv := make(map[string]string)
	for _, env := range osEnv {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			originalEnv[parts[0]] = parts[1]
		}
	}
	return BuildData{
		containers: containerData{
			containers: make(map[string]*narwhal.Container),
		},
		environment: make(map[string]string),
		originalEnv: originalEnv,
	}
}

func (b BuildData) SetEnv(exp configExpansion, key string, value string) error {
	interpolated, err := exp.expand(value)
	if err != nil {
		return fmt.Errorf("set environment variable %s: %w", key, err)
	}
	b.environment[key] = interpolated
	return nil
}

func (b BuildData) mergedEnvironment(ctx context.Context) map[string]string {
	result := make(map[string]string)
	for k, v := range b.containers.Environment(ctx) {
		result[k] = v
	}
	for k, v := range b.environment {
		result[k] = v
	}
	return result
}

func (b BuildData) ExportEnvironmentPublicly(ctx context.Context) {
	log.Infof(ctx, "Exporting environment")
	for k, v := range b.mergedEnvironment(ctx) {
		log.Infof(ctx, " * %s = %s", k, v)
		os.Setenv(k, v)
	}
}

func (b BuildData) UnexportEnvironmentPublicly(ctx context.Context) {
	log.Infof(ctx, "Unexporting environment")
	for k := range b.mergedEnvironment(ctx) {
		if _, exists := b.originalEnv[k]; exists {
			os.Setenv(k, b.originalEnv[k])
		} else {
			log.Infof(ctx, "Unsetting %s", k)
			os.Unsetenv(k)
		}
	}
}

func (b BuildData) EnvironmentVariables(ctx context.Context) []string {
	result := make([]string, 0)
	for k, v := range b.mergedEnvironment(ctx) {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
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
