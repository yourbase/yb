package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/matishsiao/goInfo"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/johnewart/subcommands"

	log "github.com/sirupsen/logrus"

	ybconfig "github.com/yourbase/yb/config"
	. "github.com/yourbase/yb/packages"
	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
	. "github.com/yourbase/yb/workspace"
)

const TIME_FORMAT = "15:04:05 MST"

type BuildCmd struct {
	ExecPrefix       string
	NoContainer      bool
	DependenciesOnly bool
}

type BuildConfiguration struct {
	Target           BuildTarget
	TargetDir        string
	Sandboxed        bool
	ExecPrefix       string
	ForceNoContainer bool
	TargetPackage    Package
	BuildData        BuildData
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
}

func (b *BuildCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	startTime := time.Now()

	log.Infof(" === BUILD HOST ===")
	gi := goInfo.GetInfo()
	gi.VarDump()

	log.Infof("Build started at %s", startTime.Format(TIME_FORMAT))
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
	var targetPackage Package

	// check if we're just a package
	if PathExists(MANIFEST_FILE) {
		currentPath, _ := filepath.Abs(".")
		_, pkgName := filepath.Split(currentPath)
		pkg, err := LoadPackage(pkgName, currentPath)
		if err != nil {
			log.Infof("Error loading package '%s': %v", pkgName, err)
			return subcommands.ExitFailure
		}
		targetPackage = pkg
	} else {

		workspace, err := LoadWorkspace()

		if err != nil {
			log.Infof("No package here, and no workspace, nothing to build!")
			return subcommands.ExitFailure
		}

		pkg, err := workspace.TargetPackage()
		if err != nil {
			log.Infof("Can't load workspace's target package: %v", err)
			return subcommands.ExitFailure
		}

		targetPackage = pkg
	}

	// Determine build target
	buildTargetName := "default"

	if len(f.Args()) > 0 {
		buildTargetName = f.Args()[0]
	}

	targetDir := targetPackage.Path

	log.Infof("Building package %s in %s...", targetPackage.Name, targetDir)
	manifest := targetPackage.Manifest

	log.Infof("Checksum of dependencies: %s", manifest.BuildDependenciesChecksum())

	//workspace.SetupEnv()
	log.Infof("\n === ENVIRONMENT SETUP ===\n")
	setupTimer, err := SetupBuildDependencies(targetPackage)

	if err != nil {
		log.Infof("Error setting up dependencies: %v", err)
		return subcommands.ExitFailure
	}

	if b.DependenciesOnly {
		log.Infof("Only installing dependencies; done!")
		return subcommands.ExitSuccess
	}

	var targetTimers []TargetTimer
	var buildError error

	log.Infof("\n\n === BUILD ===\n")
	targetTimers = append(targetTimers, setupTimer)

	config := BuildConfiguration{
		ExecPrefix:       b.ExecPrefix,
		TargetDir:        targetDir,
		ForceNoContainer: b.NoContainer,
		TargetPackage:    targetPackage,
	}

	// No targets, look for default build stanza
	if len(manifest.BuildTargets) == 0 {
		target := manifest.Build
		if len(target.Commands) == 0 {
			buildError = fmt.Errorf("Default build command has no steps and no targets described")
		} else {
			log.Infof("Building target %s in %s...", buildTargetName, targetDir)
			config.Target = target
			config.Sandboxed = manifest.IsTargetSandboxed(target)
			stepTimers, err := DoBuild(config)
			buildError = err
			targetTimers = append(targetTimers, TargetTimer{Name: target.Name, Timers: stepTimers})
		}
	} else {
		// Named target, look for that and resolve it
		buildTargets, err := manifest.ResolveBuildTargets(buildTargetName)

		if err != nil {
			fmt.Println(err)
			log.Infof("Valid build targets: %s", strings.Join(manifest.BuildTargetList(), ", "))
			return subcommands.ExitFailure
		}

		log.Infof("Going to build targets in the following order: ")
		for _, target := range buildTargets {
			log.Infof("   - %s", target.Name)
		}

		var buildStepTimers []CommandTimer
		for _, target := range buildTargets {
			if buildError == nil {
				config.Target = target
				config.Sandboxed = manifest.IsTargetSandboxed(target)
				buildStepTimers, buildError = DoBuild(config)
				targetTimers = append(targetTimers, TargetTimer{Name: target.Name, Timers: buildStepTimers})
			}
		}
	}

	endTime := time.Now()
	buildTime := endTime.Sub(startTime)

	log.Infof("\nBuild finished at %s, taking %s\n", endTime.Format(TIME_FORMAT), buildTime)
	fmt.Println()
	log.Infof("%15s%15s%15s%24s   %s", "Start", "End", "Elapsed", "Target", "Command")
	for _, timer := range targetTimers {
		for _, step := range timer.Timers {
			elapsed := step.EndTime.Sub(step.StartTime)
			log.Infof("%15s%15s%15s%24s   %s",
				step.StartTime.Format(TIME_FORMAT),
				step.EndTime.Format(TIME_FORMAT),
				elapsed,
				timer.Name,
				step.Command)
		}
	}
	log.Infof("%15s%15s%15s   %s", "", "", buildTime, "TOTAL")

	if buildError != nil {
		fmt.Println("\n\n -- BUILD FAILED -- ")
		log.Infof("\nBuild terminated with the following error: %v\n", buildError)
	} else {
		fmt.Println("\n\n -- BUILD SUCCEEDED -- ")
	}

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

func DoBuild(config BuildConfiguration) ([]CommandTimer, error) {
	target := config.Target
	targetPackage := config.TargetPackage

	log.Infof("\n\n -- Build target: %s -- \n\n", target.Name)

	// Set any environment variables as the last thing (override things we do in case people really want to do this)
	// XXX: Should we though?
	// XXX: In a perfect world we should force sandboxing by resetting all environment variables
	// XXX: Saving old state and resetting it after for now
	buildData := NewBuildData()
	config.BuildData = buildData

	var stepTimers []CommandTimer
	var buildError error
	var sc *ServiceContext

	if !config.ForceNoContainer {
		containers := target.Dependencies.Containers

		if len(containers) > 0 {
			var err error
			contextId := fmt.Sprintf("%s-%s", targetPackage.Name, target.Name)
			log.Infof("Starting %d containers with context id %s...", len(containers), contextId)
			sc, err = NewServiceContextWithId(contextId, targetPackage, containers)
			if err != nil {
				return stepTimers, fmt.Errorf("Couldn't create service context for dependencies: %v", err)
			}

			if err = sc.StandUp(); err != nil {
				return stepTimers, fmt.Errorf("Couldn't start dependencies: %v", err)
			}

			buildData.Containers.ServiceContext = sc
		}
	}

	for _, envString := range target.Environment {
		parts := strings.Split(envString, "=")
		key := parts[0]
		value := parts[1]
		buildData.SetEnv(key, value)
	}

	if target.Container.Image != "" && !config.ForceNoContainer {
		fmt.Println("Executing build steps in container")
		target.Container.Command = "tail -f /dev/null"

		u, _ := user.Current()

		containerOpts := BuildContainerOpts{
			ContainerOpts: target.Container,
			Package:       targetPackage,
			ExecUserId:    u.Uid,
			ExecGroupId:   u.Gid,
			MountPackage:  true,
		}

		stepTimers, buildError = RunCommandsInContainer(config, containerOpts)
	} else {
		// Do the commands
		log.Infof("Executing build steps...")
		buildData.ExportEnvironmentPublicly()
		stepTimers, buildError = RunCommands(config)
	}

	if sc != nil {
		log.Infof("Cleaning up containers...")
		if err := sc.TearDown(); err != nil {
			log.Infof("Problem tearing down containers: %v", err)
		}
	}

	return stepTimers, buildError

}

func RunCommandsInContainer(config BuildConfiguration, containerOpts BuildContainerOpts) ([]CommandTimer, error) {
	stepTimes := make([]CommandTimer, 0)
	target := config.Target

	// Perform build inside a container
	image := target.Container.Image
	log.Infof("Invoking build in a container: %s", image)

	var buildContainer BuildContainer

	existing, err := FindContainer(containerOpts)

	if err != nil {
		log.Infof("Failed trying to find container: %v", err)
		return stepTimes, err
	}

	if existing != nil {
		log.Infof("Found existing container %s, removing...", existing.Id)
		// Try stopping it first if needed
		_ = existing.Stop(0)
		if err = RemoveContainerById(existing.Id); err != nil {
			log.Infof("Unable to remove existing container: %v", err)
			return stepTimes, err
		}
	}

	buildContainer, err = NewContainer(containerOpts)
	defer buildContainer.Stop(0)

	if err != nil {
		log.Infof("Error creating build container: %v", err)
		return stepTimes, err
	}

	if err := buildContainer.Start(); err != nil {
		log.Infof("Unable to start container %s: %v", buildContainer.Id, err)
		return stepTimes, err
	}

	log.Infof("Building in container: %s", buildContainer.Id)

	containerWorkDir := "/workspace"
	if containerOpts.ContainerOpts.WorkDir != "" {
		containerWorkDir = containerOpts.ContainerOpts.WorkDir
	}

	for _, cmdString := range target.Commands {
		stepStartTime := time.Now()
		if len(config.ExecPrefix) > 0 {
			cmdString = fmt.Sprintf("%s %s", config.ExecPrefix, cmdString)
		}

		log.Infof("Running %s in the container in directory %s", cmdString, containerWorkDir)

		if err := buildContainer.ExecToStdout(cmdString, containerWorkDir); err != nil {
			log.Infof("Failed to run %s: %v", cmdString, err)
			return stepTimes, fmt.Errorf("Aborting build, unable to run %s: %v", cmdString, err)
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

	}

	return stepTimes, nil
}

func RunCommands(config BuildConfiguration) ([]CommandTimer, error) {

	stepTimes := make([]CommandTimer, 0)

	target := config.Target
	sandboxed := config.Sandboxed
	targetDir := config.TargetDir

	for _, cmdString := range target.Commands {
		var stepError error

		stepStartTime := time.Now()
		if len(config.ExecPrefix) > 0 {
			cmdString = fmt.Sprintf("%s %s", config.ExecPrefix, cmdString)
		}

		if strings.HasPrefix(cmdString, "cd ") {
			parts := strings.SplitN(cmdString, " ", 2)
			dir := filepath.Join(targetDir, parts[1])
			//log.Infof("Chdir to %s", dir)
			//os.Chdir(dir)
			targetDir = dir
		} else {
			if target.Root != "" {
				log.Infof("Build root is %s", target.Root)
				targetDir = filepath.Join(targetDir, target.Root)
			}

			if sandboxed {
				fmt.Println("Running build in a sandbox!")
				if err := ExecInSandbox(cmdString, targetDir); err != nil {
					log.Infof("Failed to run %s: %v", cmdString, err)
					stepError = err
				}
			} else {
				if err := ExecToStdout(cmdString, targetDir); err != nil {
					log.Infof("Failed to run %s: %v", cmdString, err)
					stepError = err
				}
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
	fmt.Println("Uploading build logs...")
	buildLog := BuildLog{
		Contents: buf.String(),
	}
	jsonData, _ := json.Marshal(buildLog)
	resp, err := postJsonToApi("/buildlogs", jsonData)

	if err != nil {
		log.Infof("Couldn't upload logs: %v", err)
		return
	}

	if resp.StatusCode != 200 {
		log.Infof("Status code uploading log: %d", resp.StatusCode)
		return
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Infof("Couldn't read response body: %s", err)
			return
		}

		var buildLog BuildLog
		err = json.Unmarshal(body, &buildLog)
		if err != nil {
			log.Infof("Failed to parse response: %v", err)
			return
		}

		logViewPath := fmt.Sprintf("/buildlogs/%s", buildLog.UUID)
		buildLogUrl, err := ybconfig.ManagementUrl(logViewPath)

		if err != nil {
			log.Infof("Unable to determine build log url: %v", err)
		}

		log.Infof("View your build log here: %s", buildLogUrl)
	}

}

func SetupBuildDependencies(pkg Package) (TargetTimer, error) {

	startTime := time.Now()
	// Ensure build deps are :+1:
	stepTimers, err := pkg.SetupBuildDependencies()

	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)

	log.Infof("Dependency setup happened in %s", elapsedTime)

	setupTimer := TargetTimer{
		Name:   "dependency_setup",
		Timers: stepTimers,
	}

	return setupTimer, err
}

type ContainerData struct {
	ServiceContext *ServiceContext
}

func (c ContainerData) IP(label string) string {
	if c.ServiceContext != nil {
		if buildContainer, ok := c.ServiceContext.BuildContainers[label]; ok {
			if ipv4, err := buildContainer.IPv4Address(); err == nil {
				return ipv4
			}
		}
	}

	return ""
}

type BuildData struct {
	Containers  ContainerData
	Environment map[string]string
}

func NewBuildData() BuildData {
	return BuildData{
		Containers:  ContainerData{},
		Environment: make(map[string]string),
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

func (b BuildData) ExportEnvironmentPublicly() {
	log.Infof("Exporting environment")
	for k, v := range b.Environment {
		log.Infof(" * %s = %s", k, v)
		os.Setenv(k, v)
	}
}
