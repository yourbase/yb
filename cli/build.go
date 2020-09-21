package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/matishsiao/goInfo"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/johnewart/archiver"
	"github.com/johnewart/narwhal"
	"github.com/johnewart/subcommands"

	ybconfig "github.com/yourbase/yb/config"
	. "github.com/yourbase/yb/packages"
	. "github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/types"
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
	Target           BuildTarget
	TargetDir        string
	ExecPrefix       string
	ForceNoContainer bool
	TargetPackage    Package
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
	return `build`
}

func (b *BuildCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&b.NoContainer, "no-container", false, "Bypass container even if specified")
	f.BoolVar(&b.DependenciesOnly, "deps-only", false, "Install only dependencies, don't do anything else")
	f.StringVar(&b.ExecPrefix, "exec-prefix", "", "Add a prefix to all executed commands (useful for timing or wrapping things)")
	f.BoolVar(&b.ReuseContainers, "reuse-containers", true, "Reuse the container for building")
	f.BoolVar(&b.NoSideContainer, "no-side-container", false, "Don't run/create any side container")
}

func (b *BuildCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	startTime := time.Now()

	if InsideTheMatrix() {
		log.StartSection("BUILD CONTAINER", "CONTAINER")
	} else {
		log.StartSection("BUILD HOST", "HOST")
	}
	gi := goInfo.GetInfo()
	gi.VarDump()
	log.EndSection()

	log.StartSection("BUILD PACKAGE SETUP", "SETUP")
	log.Infof("Build started at %s", startTime.Format(TIME_FORMAT))

	targetPackage, err := GetTargetPackage()
	if err != nil {
		log.Errorf("%v", err)
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
		log.Errorf("Could not compute build target '%s': %v", buildTargetName, err)
		log.Infof("Valid build targets: %s", strings.Join(manifest.BuildTargetList(), ", "))
		return subcommands.ExitFailure
	}

	primaryTarget, err := manifest.BuildTarget(buildTargetName)
	if err != nil {
		log.Errorf("Couldn't get primary build target '%s' specs: %v", buildTargetName, err)
		return subcommands.ExitFailure
	}

	buildData := NewBuildData()

	if !b.NoContainer {
		target := primaryTarget
		// Setup dependencies
		containers := target.Dependencies.Containers

		if err := narwhal.DockerClient().Ping(); err != nil {
			log.Error("Couldn't connect to Docker daemon. Try installing Docker Desktop: https://hub.docker.com/search/?type=edition&offering=community")
			return subcommands.ExitFailure
		}
		contextId := fmt.Sprintf("%s-%s", targetPackage.Name, target.Name)
		sc, err := narwhal.NewServiceContextWithId(contextId, workDir)
		if err != nil {
			log.Errorf("Couldn't create service context for dependencies: %v", err)
			return subcommands.ExitFailure
		}

		buildData.Containers.ServiceContext = sc

		if len(containers) > 0 && !b.NoSideContainer {
			log.Infof("Starting %d containers with context id %s...", len(containers), contextId)
			if !b.ReuseContainers {
				log.Infof("Not reusing containers, will teardown existing ones and clean up after ourselves")
				defer Cleanup(buildData)
				if err := sc.TearDown(); err != nil {
					log.Errorf("Couldn't terminate existing containers: %v", err)
					// FAIL?
				}
			}

			for _, c := range containers {
				_, err := sc.StartContainer(c)
				if err != nil {
					log.Errorf("Couldn't start dependencies: %v", err)
					return subcommands.ExitFailure
				}
			}

		}
	}

	// Do this after the containers are up
	for _, envString := range primaryTarget.Environment {
		parts := strings.SplitN(envString, "=", 2)
		if len(parts) != 2 {
			log.Warnf("'%s' doesn't look like an environment variable", envString)
			continue
		}
		buildData.SetEnv(parts[0], parts[1])
	}

	_, exists := os.LookupEnv("YB_NO_CONTAINER_BUILDS")
	// Should we build in a container?
	if !b.NoContainer && !primaryTarget.HostOnly && !exists {
		log.StartSection("BUILD IN CONTAINER", "BCONTAINER")
		log.Infof("Executing build steps in container")

		target := primaryTarget
		containerDef := target.Container
		containerDef.Command = "/usr/bin/tail -f /dev/null"

		// Append build environment variables
		//container.Environment = append(container.Environment, buildData.EnvironmentVariables()...)
		containerDef.Environment = []string{}

		// Add package to mounts @ /workspace
		sourceMapDir := "/workspace"
		if containerDef.WorkDir != "" {
			sourceMapDir = containerDef.WorkDir
		}

		log.Infof("Will mount package %s at %s in container", targetPackage.Path, sourceMapDir)
		mount := fmt.Sprintf("%s:%s", targetPackage.Path, sourceMapDir)
		containerDef.Mounts = append(containerDef.Mounts, mount)

		if false {
			u, _ := user.Current()
			fmt.Printf("U! %s", u.Uid)
		}

		// Do our build in a container - don't do the build locally
		buildErr := b.BuildInsideContainer(target, containerDef, buildData, b.ExecPrefix, b.ReuseContainers)
		if buildErr != nil {
			log.Errorf("Unable to build %s inside container: %v", target.Name, buildErr)
			return subcommands.ExitFailure
		}

		return subcommands.ExitSuccess
	}

	// Do the build!
	targetDir := targetPackage.Path

	log.Infof("Building package %s in %s...", targetPackage.Name, targetDir)
	log.Infof("Checksum of dependencies: %s", manifest.BuildDependenciesChecksum())

	log.EndSection()

	if len(primaryTarget.Dependencies.Containers) > 0 {
		log.StartSection("CONTAINER SETUP", "CONTAINER")
		log.Info("")
		log.Info("")
		log.Infof("Available side containers:")
		for label, c := range primaryTarget.Dependencies.Containers {
			ipv4 := buildData.Containers.IP(label)
			log.Infof("  * %s (using %s) has IP address %s", label, c.ImageNameWithTag(), ipv4)
		}
		log.EndSection()
	}

	var targetTimers []TargetTimer
	var buildError error

	log.StartSection("BUILD", "BUILD")

	config := BuildConfiguration{
		ExecPrefix:       b.ExecPrefix,
		TargetDir:        targetDir,
		ForceNoContainer: b.NoContainer,
		TargetPackage:    targetPackage,
		ReuseContainers:  b.ReuseContainers,
		BuildData:        buildData,
	}

	log.Infof("Going to build targets in the following order: ")
	for _, target := range buildTargets {
		log.Infof("   - %s", target.Name)
	}

	var buildStepTimers []CommandTimer
	for _, target := range buildTargets {
		depsTimer, err := SetupBuildDependencies(targetPackage, target)
		if err != nil {
			log.Infof("Error setting up dependencies for target %s: %v", target.Name, err)
			return subcommands.ExitFailure
		}
		targetTimers = append(targetTimers, depsTimer)

		if !b.DependenciesOnly && buildError == nil {
			log.SubSection(fmt.Sprintf("Build target: %s", target.Name))
			config.Target = target
			buildData.ExportEnvironmentPublicly()
			log.Infof("Executing build steps...")
			buildStepTimers, buildError = RunCommands(config)
			targetTimers = append(targetTimers, TargetTimer{Name: target.Name, Timers: buildStepTimers})
			buildData.UnexportEnvironmentPublicly()
		}
		if buildError != nil {
			break
		}
	}
	if b.DependenciesOnly {
		log.Infof("Only installing dependencies; done!")
		return subcommands.ExitSuccess
	}

	endTime := time.Now()
	buildTime := endTime.Sub(startTime)
	time.Sleep(100 * time.Millisecond)

	log.Info("")
	log.Infof("Build finished at %s, taking %s", endTime.Format(TIME_FORMAT), buildTime)
	log.Info("")
	log.Infof("%15s%15s%15s%24s   %s", "Start", "End", "Elapsed", "Target", "Command")
	for _, timer := range targetTimers {
		for _, step := range timer.Timers {
			elapsed := step.EndTime.Sub(step.StartTime).Truncate(time.Microsecond)
			log.Infof("%15s%15s%15s%24s   %s",
				step.StartTime.Format(TIME_FORMAT),
				step.EndTime.Format(TIME_FORMAT),
				elapsed,
				timer.Name,
				step.Command)
		}
	}
	log.Infof("%15s%15s%15s   %s", "", "", buildTime.Truncate(time.Millisecond), "TOTAL")

	if buildError != nil {
		log.SubSection("BUILD FAILED")
		log.Errorf("Build terminated with the following error: %v", buildError)
	} else {
		log.SubSection("BUILD SUCCEEDED")
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
		UploadBuildLogsToAPI(&buf)
	}

	if buildError != nil {
		return subcommands.ExitFailure
	}

	// No errors, :+1:
	return subcommands.ExitSuccess
}

func Cleanup(b BuildData) {
	if b.Containers.ServiceContext != nil {
		log.Infof("Cleaning up containers...")
		if err := b.Containers.ServiceContext.TearDown(); err != nil {
			log.Warnf("Problem tearing down containers: %v", err)
		}
	}
}

func (b *BuildCmd) BuildInsideContainer(target BuildTarget, containerDef narwhal.ContainerDefinition, buildData BuildData, execPrefix string, reuseOldContainer bool) error {
	// Perform build inside a container
	image := target.Container.Image
	log.Infof("Using container image: %s", image)
	buildContainer, err := buildData.Containers.ServiceContext.StartContainer(containerDef)

	if err != nil {
		return fmt.Errorf("Failed trying to find container: %v", err)
	}

	if buildContainer != nil {
		log.Infof("Found existing container %s", buildContainer.Id[0:12])

		_ = buildContainer.Stop(0)

		if !reuseOldContainer {
			log.Infof("Elected to not re-use containers, will remove the container")
			// Try stopping it first if needed
			if err = narwhal.RemoveContainerById(buildContainer.Id); err != nil {
				return fmt.Errorf("Unable to remove existing container: %v", err)
			}
		}
	}

	if buildContainer != nil && reuseOldContainer {
		log.Infof("Reusing existing build container %s", buildContainer.Id[0:12])
	} else {
		log.Infof("Creating a new build container...")
		if c, err := buildData.Containers.ServiceContext.StartContainer(containerDef); err != nil {
			return fmt.Errorf("Couldn't create a new build container: %v", err)
		} else {
			buildContainer = c
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		buildContainer.Stop(0)
		os.Exit(0)
	}()

	defer buildContainer.Stop(0)

	if err != nil {
		return fmt.Errorf("Error creating build container: %v", err)
	}

	if err := buildContainer.Start(); err != nil {
		return fmt.Errorf("Unable to start container %s: %v", buildContainer.Id, err)
	}

	// Set container work dir
	containerWorkDir := "/workspace"
	if containerDef.WorkDir != "" {
		containerWorkDir = containerDef.WorkDir
	}

	log.Infof("Building from %s in container: %s", containerWorkDir, buildContainer.Id)

	// Local path to binary that we want to inject
	// TODO: this only supports Linux containers
	localYbPath := ""
	devMode := false

	// If this is development mode, use the YB binary currently running
	// We can't do this by default because this only works if the host is
	// Linux so we might as well behave the same on all platforms
	// If not development mode, download the binary from the distribution channel
	if b.Version == "DEVELOPMENT" {
		if runtime.GOOS == "linux" {
			// TODO: If we support building inside a container that's not Linux we will want to do something different
			log.Infof("Development version in use, will upload development binary to the container")
			devMode = true
		}
	}

	if devMode {
		if p, err := os.Executable(); err != nil {
			return fmt.Errorf("Can't determine local path to YB: %v", err)
		} else {
			localYbPath = p
		}
	} else {
		if p, err := DownloadYB(); err != nil {
			return fmt.Errorf("Couldn't download YB: %v", err)
		} else {
			localYbPath = p
		}
	}

	// Upload and update CLI
	log.Infof("Uploading YB from %s to /yb", localYbPath)
	if err := buildContainer.UploadFile(localYbPath, "yb", "/"); err != nil {
		return fmt.Errorf("Unable to upload YB to container: %v", err)
	}

	if !devMode {
		// Default to whatever channel being used, unless told otherwise
		// TODO: Make the download URL used for downloading track the latest stable
		ybChannel := b.Channel

		// Overwrites local configuration
		if envChannel, exists := os.LookupEnv("YB_UPDATE_CHANNEL"); exists {
			ybChannel = envChannel
		}

		log.Infof("Updating YB in container from channel %s", ybChannel)
		updateCmd := fmt.Sprintf("/yb update -channel=%s", ybChannel)
		if err := buildContainer.ExecToStdoutWithEnv(updateCmd, containerWorkDir, buildData.EnvironmentVariables()); err != nil {
			return fmt.Errorf("Aborting build, unable to run %s: %v", updateCmd, err)
		}
	}

	cmdString := "/yb build"

	if len(execPrefix) > 0 {
		cmdString = fmt.Sprintf("%s -exec-prefix %s", cmdString, execPrefix)
	}

	cmdString = fmt.Sprintf("%s -no-container %s", cmdString, target.Name)

	log.Infof("Running %s in the container in directory %s", cmdString, containerWorkDir)

	if err := buildContainer.ExecToStdoutWithEnv(cmdString, containerWorkDir, buildData.EnvironmentVariables()); err != nil {
		log.Errorf("Failed to run %s: %v", cmdString, err)
		return fmt.Errorf("Aborting build, unable to run %s: %v", cmdString, err)
	}

	// Make sure our goroutine gets this from stdout
	// TODO: There must be a better way...
	time.Sleep(10 * time.Millisecond)

	return nil
}

func RunCommands(config BuildConfiguration) ([]CommandTimer, error) {

	stepTimes := make([]CommandTimer, 0)

	target := config.Target
	targetDir := config.TargetDir
	if target.Root != "" {
		log.Infof("Build root is %s", target.Root)
		targetDir = filepath.Join(targetDir, target.Root)
	}

	for _, cmdString := range target.Commands {
		var stepError error

		stepStartTime := time.Now()
		if strings.HasPrefix(cmdString, "cd ") {
			parts := strings.SplitN(cmdString, " ", 2)
			dir := filepath.Join(targetDir, parts[1])
			//log.Infof("Chdir to %s", dir)
			//os.Chdir(dir)
			targetDir = dir
		} else {
			if len(config.ExecPrefix) > 0 {
				cmdString = fmt.Sprintf("%s %s", config.ExecPrefix, cmdString)
			}

			if err := ExecToStdout(cmdString, targetDir); err != nil {
				log.Errorf("Failed to run %s: %v", cmdString, err)
				stepError = err
			}
		}

		stepEndTime := time.Now()
		stepTotalTime := stepEndTime.Sub(stepStartTime)

		log.Infof("Completed '%s' in %s", cmdString, stepTotalTime)

		cmdTimer := CommandTimer{
			Command:   cmdString,
			StartTime: stepStartTime,
			EndTime:   stepEndTime,
		}

		stepTimes = append(stepTimes, cmdTimer)
		// Make sure our goroutine gets this from stdout
		// TODO: There must be a better way...
		time.Sleep(10 * time.Millisecond)
		if stepError != nil {
			return stepTimes, stepError
		}
	}

	return stepTimes, nil
}

func UploadBuildLogsToAPI(buf *bytes.Buffer) {
	log.Infof("Uploading build logs...")
	buildLog := BuildLog{
		Contents: buf.String(),
	}
	jsonData, _ := json.Marshal(buildLog)
	resp, err := postJsonToApi("/buildlogs", jsonData)

	if err != nil {
		log.Errorf("Couldn't upload logs: %v", err)
		return
	}

	if resp.StatusCode != 200 {
		log.Warnf("Status code uploading log: %d", resp.StatusCode)
		return
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Errorf("Couldn't read response body: %s", err)
			return
		}

		var buildLog BuildLog
		err = json.Unmarshal(body, &buildLog)
		if err != nil {
			log.Errorf("Failed to parse response: %v", err)
			return
		}

		logViewPath := fmt.Sprintf("/buildlogs/%s", buildLog.UUID)
		buildLogUrl, err := ybconfig.ManagementUrl(logViewPath)

		if err != nil {
			log.Errorf("Unable to determine build log url: %v", err)
		}

		log.Infof("View your build log here: %s", buildLogUrl)
	}

}

func SetupBuildDependencies(pkg Package, target BuildTarget) (TargetTimer, error) {

	startTime := time.Now()
	// Ensure build deps are :+1:
	stepTimers, err := pkg.SetupBuildDependencies(target)

	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)

	log.Infof("Dependency setup for %s happened in %s", target.Name, elapsedTime)

	setupTimer := TargetTimer{
		Name:   "dependency_setup",
		Timers: stepTimers,
	}

	return setupTimer, err
}

// Build metadata; used for things like interpolation in environment variables
type ContainerData struct {
	ServiceContext *narwhal.ServiceContext
}

func (c ContainerData) IP(label string) string {
	// Check service context
	if c.ServiceContext != nil {
		if buildContainer, ok := c.ServiceContext.Containers[label]; ok {
			if ipv4, err := buildContainer.IPv4Address(); err == nil {
				return ipv4
			}
		}
	}

	// Look for environment variable (injected into containers)
	envKey := fmt.Sprintf("YB_CONTAINER_%s_IP", strings.ToUpper(label))
	if ip, exists := os.LookupEnv(envKey); exists {
		return ip
	}

	return ""
}

func (c ContainerData) Environment() map[string]string {
	result := make(map[string]string)
	if c.ServiceContext != nil {
		for label, container := range c.ServiceContext.Containers {
			if ipv4, err := container.IPv4Address(); err == nil {
				key := fmt.Sprintf("YB_CONTAINER_%s_IP", strings.ToUpper(label))
				result[key] = ipv4
			}
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
		Containers:  ContainerData{},
		Environment: make(map[string]string),
		originalEnv: originalEnv,
	}
}

func (b BuildData) SetEnv(key string, value string) {
	interpolated, err := TemplateToString(value, b)
	if err != nil {
		b.Environment[key] = value
	} else {
		b.Environment[key] = interpolated
	}
}

func (b BuildData) mergedEnvironment() map[string]string {
	result := make(map[string]string)
	for k, v := range b.Containers.Environment() {
		result[k] = v
	}
	for k, v := range b.Environment {
		result[k] = v
	}
	return result
}

func (b BuildData) ExportEnvironmentPublicly() {
	log.Infof("Exporting environment")
	for k, v := range b.mergedEnvironment() {
		log.Infof(" * %s = %s", k, v)
		os.Setenv(k, v)
	}
}

func (b BuildData) UnexportEnvironmentPublicly() {
	log.Infof("Unexporting environment")
	for k := range b.mergedEnvironment() {
		if _, exists := b.originalEnv[k]; exists {
			os.Setenv(k, b.originalEnv[k])
		} else {
			log.Infof("Unsetting %s", k)
			os.Unsetenv(k)
		}
	}
}

func (b BuildData) EnvironmentVariables() []string {
	result := make([]string, 0)
	for k, v := range b.mergedEnvironment() {
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

// TODO: non-linux things too if we ever support non-Linux containers
// TODO: on non-Linux platforms we shouldn't constantly try to re-download it
func DownloadYB() (string, error) {
	// Stick with this version, we can track some relatively recent version because
	// we will just update anyway so it doesn't need to be super-new unless we broke something
	downloadUrl := "https://bin.equinox.io/a/7G9uDXWDjh8/yb-0.0.39-linux-amd64.tar.gz"
	binarySha256 := "3e21a9c98daa168ea95a5be45d14408c18688b5eea211d7936f6cd013bd23210"
	cachePath := CacheDir()
	tmpPath := filepath.Join(cachePath, ".yb-tmp")
	ybPath := filepath.Join(tmpPath, "yb")
	MkdirAsNeeded(tmpPath)

	if PathExists(ybPath) {
		checksum, err := sha256File(ybPath)
		// Checksum match? We're good to go
		if err == nil && checksum == binarySha256 {
			return ybPath, nil
		}

		log.Infof("Local binary checksum mis-match, re-downloading YB")
	}

	// Couldn't tell, check if we need to and download the archive
	localFile, err := DownloadFileWithCache(downloadUrl)
	if err != nil {
		return "", err
	}

	err = archiver.Unarchive(localFile, tmpPath)
	if err != nil {
		return "", fmt.Errorf("Couldn't decompress: %v", err)
	}

	return ybPath, nil
}
