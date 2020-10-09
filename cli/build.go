package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/johnewart/subcommands"
	"github.com/matishsiao/goInfo"
	"github.com/yourbase/commons/xcontext"
	"github.com/yourbase/narwhal"
	ybconfig "github.com/yourbase/yb/config"
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
	Channel          string
	Version          string
	CommitSHA        string
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

type BuildLog struct {
	Contents string `json:"contents"`
	UUID     string `json:"uuid"`
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
	buildTraces := new(traceSink)
	tp, err := sdktrace.NewProvider(sdktrace.WithSyncer(buildTraces))
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	global.SetTraceProvider(tp)

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
	workDir := targetPackage.BuildRoot()

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

	if !b.NoContainer {
		dockerClient, err := docker.NewVersionedClient("unix:///var/run/docker.sock", "1.39")
		if err != nil {
			log.Errorf(ctx, "%v", err)
			return subcommands.ExitFailure
		}
		target := primaryTarget
		// Setup dependencies
		containers := target.Dependencies.Containers

		if err := narwhal.DockerClient().Ping(); err != nil {
			log.Errorf(ctx, "Couldn't connect to Docker daemon. Try installing Docker Desktop: https://hub.docker.com/search/?type=edition&offering=community")
			return subcommands.ExitFailure
		}
		contextId := fmt.Sprintf("%s-%s", targetPackage.Name, target.Name)
		sc, err := narwhal.NewServiceContextWithId(ctx, dockerClient, contextId, workDir)
		if err != nil {
			log.Errorf(ctx, "Couldn't create service context for dependencies: %v", err)
			return subcommands.ExitFailure
		}

		buildData.Containers.ServiceContext = sc

		if len(containers) > 0 && !b.NoSideContainer {
			log.Infof(ctx, "Starting %d containers with context id %s...", len(containers), contextId)
			if !b.ReuseContainers {
				log.Infof(ctx, "Not reusing containers, will teardown existing ones and clean up after ourselves")
				defer Cleanup(xcontext.IgnoreDeadline(ctx), buildData)
				if err := sc.TearDown(xcontext.IgnoreDeadline(ctx)); err != nil {
					log.Errorf(ctx, "Couldn't terminate existing containers: %v", err)
					// FAIL?
				}
			}

			for _, cd := range containers {
				c, err := sc.StartContainer(ctx, os.Stderr, cd.ToNarwhal())
				if err != nil {
					log.Errorf(ctx, "Couldn't start dependencies: %v", err)
					return subcommands.ExitFailure
				}
				buildData.Containers.Containers[cd.Label] = c
			}

		}
	}

	// Do this after the containers are up
	for _, envString := range primaryTarget.Environment {
		parts := strings.SplitN(envString, "=", 2)
		if len(parts) != 2 {
			log.Warnf(ctx, "'%s' doesn't look like an environment variable", envString)
			continue
		}
		buildData.SetEnv(parts[0], parts[1])
	}

	_, exists := os.LookupEnv("YB_NO_CONTAINER_BUILDS")
	// Should we build in a container?
	if !b.NoContainer && !primaryTarget.HostOnly && !exists {
		startSection("BUILD IN CONTAINER")
		log.Infof(ctx, "Executing build steps in container")

		target := primaryTarget
		containerDef := target.Container.ToNarwhal()
		containerDef.Command = "/usr/bin/tail -f /dev/null"

		// Append build environment variables
		//container.Environment = append(container.Environment, buildData.EnvironmentVariables()...)
		containerDef.Environment = []string{}

		// Add package to mounts @ /workspace
		sourceMapDir := "/workspace"
		if containerDef.WorkDir != "" {
			sourceMapDir = containerDef.WorkDir
		}

		log.Infof(ctx, "Will mount package %s at %s in container", targetPackage.Path, sourceMapDir)
		mount := fmt.Sprintf("%s:%s", targetPackage.Path, sourceMapDir)
		containerDef.Mounts = append(containerDef.Mounts, mount)

		// Do our build in a container - don't do the build locally
		err := b.BuildInsideContainer(ctx, target, containerDef, buildData, b.ExecPrefix, b.ReuseContainers)
		if err != nil {
			log.Errorf(ctx, "Unable to build %s inside container: %v", target.Name, err)
			return subcommands.ExitFailure
		}

		return subcommands.ExitSuccess
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
			ipv4 := buildData.Containers.IP(ctx, label)
			log.Infof(ctx, "  * %s (using %s) has IP address %s", label, c.ToNarwhal().ImageNameWithTag(), ipv4)
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
		buildError = targetPackage.SetupBuildDependencies(targetCtx, target)
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

	time.Sleep(100 * time.Millisecond)

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

	// Output duplication start
	realStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputs := io.MultiWriter(realStdout)
	var buf bytes.Buffer
	uploadBuildLogs := ybconfig.ShouldUploadBuildLogs()

	if uploadBuildLogs {
		outputs = io.MultiWriter(realStdout, &buf)
	}

	c := make(chan bool)

	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		for {
			select {
			case <-c:
				return
			case <-time.After(100 * time.Millisecond):
				io.Copy(outputs, r)
			}
		}
	}()
	defer func() {
		w.Close()
		io.Copy(outputs, r)
		close(c)
		r.Close()
	}()
	// Output duplication end

	time.Sleep(10 * time.Millisecond)
	// Reset stdout
	//os.Stdout = realStdout

	if uploadBuildLogs {
		UploadBuildLogsToAPI(ctx, &buf)
	}

	if buildError != nil {
		return subcommands.ExitFailure
	}

	// No errors, :+1:
	return subcommands.ExitSuccess
}

func Cleanup(ctx context.Context, b BuildData) {
	if b.Containers.ServiceContext != nil {
		log.Infof(ctx, "Cleaning up containers...")
		if err := b.Containers.ServiceContext.TearDown(ctx); err != nil {
			log.Warnf(ctx, "Problem tearing down containers: %v", err)
		}
	}
}

func (b *BuildCmd) BuildInsideContainer(ctx context.Context, target *types.BuildTarget, containerDef *narwhal.ContainerDefinition, buildData BuildData, execPrefix string, reuseOldContainer bool) error {
	dockerClient := buildData.Containers.ServiceContext.DockerClient
	image := containerDef.Image
	log.Infof(ctx, "Using container image: %s", image)
	buildContainer, err := buildData.Containers.ServiceContext.FindContainer(ctx, containerDef)
	if err != nil {
		return err
	}

	if buildContainer != nil {
		log.Infof(ctx, "Found existing container %s", buildContainer.Id[0:12])

		if !reuseOldContainer {
			log.Infof(ctx, "Elected to not re-use containers, will remove the container")
			// Try stopping it first if needed
			_ = dockerClient.StopContainer(buildContainer.Id, 0)
			err := dockerClient.RemoveContainer(docker.RemoveContainerOptions{
				Context: ctx,
				ID:      buildContainer.Id,
			})
			if err != nil {
				return fmt.Errorf("Unable to remove existing container: %v", err)
			}
			buildContainer = nil
		}
	}
	if buildContainer == nil {
		log.Infof(ctx, "Creating a new build container...")
		buildContainer, err = buildData.Containers.ServiceContext.StartContainer(ctx, os.Stderr, containerDef)
		if err != nil {
			return fmt.Errorf("Couldn't create a new build container: %v", err)
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		dockerClient.StopContainerWithContext(buildContainer.Id, 0, ctx)
		os.Exit(0)
	}()

	defer dockerClient.StopContainerWithContext(buildContainer.Id, 0, xcontext.IgnoreDeadline(ctx))

	if err != nil {
		return fmt.Errorf("Error creating build container: %v", err)
	}

	if err := narwhal.StartContainer(ctx, dockerClient, buildContainer.Id); err != nil {
		return fmt.Errorf("Unable to start container %s: %v", buildContainer.Id, err)
	}

	// Set container work dir
	containerWorkDir := "/workspace"
	if containerDef.WorkDir != "" {
		containerWorkDir = containerDef.WorkDir
	}

	log.Infof(ctx, "Building from %s in container: %s", containerWorkDir, buildContainer.Id)

	// If this is development mode, use the YB binary currently running
	// We can't do this by default because this only works if the host is
	// Linux so we might as well behave the same on all platforms
	// If not development mode, download the binary from the distribution channel
	devMode := false
	if b.Version == "DEVELOPMENT" && runtime.GOOS == "linux" {
		// TODO: If we support building inside a container that's not Linux we will want to do something different
		log.Infof(ctx, "Development version in use, will upload development binary to the container")
		devMode = true
	}

	if err := uploadYBToContainer(ctx, dockerClient, buildContainer, devMode); err != nil {
		return err
	}

	if !devMode {
		// Default to whatever channel being used, unless told otherwise
		// TODO: Make the download URL used for downloading track the latest stable
		ybChannel := b.Channel

		// Overwrites local configuration
		if envChannel, exists := os.LookupEnv("YB_UPDATE_CHANNEL"); exists {
			ybChannel = envChannel
		}

		log.Infof(ctx, "Updating YB in container from channel %s", ybChannel)
		err := execToStdoutWithEnv(
			ctx,
			dockerClient,
			buildContainer, []string{"/yb", "update", "-channel=" + ybChannel},
			containerWorkDir,
			buildData.EnvironmentVariables(ctx),
		)
		if err != nil {
			return err
		}
	}

	argv := []string{"/yb", "build"}
	if len(execPrefix) > 0 {
		argv = append(argv, "-exec-prefix", execPrefix)
	}
	argv = append(argv, "-no-container", target.Name)

	if err := execToStdoutWithEnv(ctx, dockerClient, buildContainer, argv, containerWorkDir, buildData.EnvironmentVariables(ctx)); err != nil {
		return err
	}

	// Make sure our goroutine gets this from stdout
	// TODO: There must be a better way...
	time.Sleep(10 * time.Millisecond)

	return nil
}

func execToStdoutWithEnv(ctx context.Context, client *docker.Client, c *narwhal.Container, argv []string, workDir string, env []string) error {
	cmdString := strings.Join(argv, " ")
	log.Infof(ctx, "Running in container directory %s: %s", workDir, cmdString)
	opts := docker.CreateExecOptions{
		Context:      ctx,
		Container:    c.Id,
		Cmd:          argv,
		WorkingDir:   workDir,
		Env:          env,
		AttachStdout: true,
		AttachStderr: true,
	}
	if c.Definition.ExecUserId != "" || c.Definition.ExecGroupId != "" {
		opts.User = c.Definition.ExecUserId + ":" + c.Definition.ExecGroupId
	}
	e, err := client.CreateExec(opts)
	if err != nil {
		return fmt.Errorf("exec %s: %w", cmdString, err)
	}
	err = client.StartExec(e.ID, docker.StartExecOptions{
		Context:      ctx,
		OutputStream: os.Stdout,
		ErrorStream:  os.Stdout,
	})
	if err != nil {
		return fmt.Errorf("exec %s: %w", cmdString, err)
	}
	return nil
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

func UploadBuildLogsToAPI(ctx context.Context, buf *bytes.Buffer) {
	log.Infof(ctx, "Uploading build logs...")
	buildLog := BuildLog{
		Contents: buf.String(),
	}
	jsonData, _ := json.Marshal(buildLog)
	resp, err := postJsonToApi("/buildlogs", jsonData)

	if err != nil {
		log.Errorf(ctx, "Couldn't upload logs: %v", err)
		return
	}

	if resp.StatusCode != 200 {
		log.Warnf(ctx, "Status code uploading log: %d", resp.StatusCode)
		return
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Errorf(ctx, "Couldn't read response body: %s", err)
			return
		}

		var buildLog BuildLog
		err = json.Unmarshal(body, &buildLog)
		if err != nil {
			log.Errorf(ctx, "Failed to parse response: %v", err)
			return
		}

		logViewPath := fmt.Sprintf("/buildlogs/%s", buildLog.UUID)
		buildLogURL, err := ybconfig.UIURL(logViewPath)

		if err != nil {
			log.Errorf(ctx, "Unable to determine build log url: %v", err)
		}

		log.Infof(ctx, "View your build log here: %s", buildLogURL)
	}

}

// Build metadata; used for things like interpolation in environment variables
type ContainerData struct {
	ServiceContext *narwhal.ServiceContext
	Containers     map[string]*narwhal.Container
}

func (c ContainerData) IP(ctx context.Context, label string) string {
	if buildContainer := c.Containers[label]; buildContainer != nil && c.ServiceContext != nil {
		ip, err := narwhal.IPv4Address(ctx, c.ServiceContext.DockerClient, buildContainer.Id)
		if err != nil {
			log.Warnf(ctx, "Obtaining address for %s container: %v", label, buildContainer)
		} else {
			return ip.String()
		}
	}

	// Look for environment variable (injected into containers)
	envKey := fmt.Sprintf("YB_CONTAINER_%s_IP", strings.ToUpper(label))
	if ip, exists := os.LookupEnv(envKey); exists {
		return ip
	}

	return ""
}

func (c ContainerData) Environment(ctx context.Context) map[string]string {
	result := make(map[string]string)
	if c.ServiceContext != nil {
		for label, container := range c.Containers {
			ipv4, err := narwhal.IPv4Address(ctx, c.ServiceContext.DockerClient, container.Id)
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
	Containers  ContainerData
	Environment map[string]string
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
		Containers: ContainerData{
			Containers: make(map[string]*narwhal.Container),
		},
		Environment: make(map[string]string),
		originalEnv: originalEnv,
	}
}

func (b BuildData) SetEnv(key string, value string) {
	interpolated, err := plumbing.TemplateToString(value, b)
	if err != nil {
		b.Environment[key] = value
	} else {
		b.Environment[key] = interpolated
	}
}

func (b BuildData) mergedEnvironment(ctx context.Context) map[string]string {
	result := make(map[string]string)
	for k, v := range b.Containers.Environment(ctx) {
		result[k] = v
	}
	for k, v := range b.Environment {
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

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func uploadYBToContainer(ctx context.Context, dockerClient *docker.Client, buildContainer *narwhal.Container, devMode bool) error {
	log.Infof(ctx, "Uploading YB to /yb")

	var localYB io.ReadCloser
	var localYBSize int64
	if devMode {
		p, err := os.Executable()
		if err != nil {
			return fmt.Errorf("upload yb to container: %w", err)
		}
		f, err := os.Open(p)
		if err != nil {
			return fmt.Errorf("upload yb to container: %w", err)
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return fmt.Errorf("upload yb to container: %w", err)
		}
		localYB = f
		localYBSize = info.Size()
	} else {
		var err error
		localYB, localYBSize, err = downloadYB(ctx)
		if err != nil {
			return fmt.Errorf("upload yb to container: %w", err)
		}
		defer localYB.Close()
	}

	err := narwhal.Upload(ctx, dockerClient, buildContainer.Id, "/yb", localYB, &tar.Header{
		Mode:     0777,
		Size:     localYBSize,
		Typeflag: tar.TypeReg,
	})
	if err != nil {
		return fmt.Errorf("upload yb to container: %w", err)
	}
	return nil
}

// TODO: non-linux things too if we ever support non-Linux containers
// TODO: on non-Linux platforms we shouldn't constantly try to re-download it
func downloadYB(ctx context.Context) (_ io.ReadCloser, size int64, err error) {
	// Stick with this version, we can track some relatively recent version because
	// we will just update anyway so it doesn't need to be super-new unless we broke something
	const downloadURL = "https://bin.equinox.io/a/7G9uDXWDjh8/yb-0.0.39-linux-amd64.tar.gz"
	archivePath, err := plumbing.DownloadFileWithCache(ctx, http.DefaultClient, downloadURL)
	if err != nil {
		return nil, 0, fmt.Errorf("download yb: %w", err)
	}
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return nil, 0, fmt.Errorf("download yb: %w", err)
	}
	defer func() {
		if err != nil {
			archiveFile.Close()
		}
	}()
	zr, err := gzip.NewReader(archiveFile)
	if err != nil {
		return nil, 0, fmt.Errorf("download yb: %s: %w", archivePath, err)
	}
	defer func() {
		if err != nil {
			zr.Close()
		}
	}()
	archive := tar.NewReader(zr)
	for {
		hdr, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("download yb: %s: %w", archivePath, err)
		}
		if hdr.Name == "yb" {
			return multiCloseReader{
				Reader:  archive,
				closers: []io.Closer{zr, archiveFile},
			}, hdr.Size, nil
		}
	}
	return nil, 0, fmt.Errorf("download yb: no 'yb' found in %s", downloadURL)
}

type multiCloseReader struct {
	io.Reader
	closers []io.Closer
}

func (mcr multiCloseReader) Close() error {
	var firstErr error
	for _, c := range mcr.closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
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
