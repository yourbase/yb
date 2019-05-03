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
	"path/filepath"
	"strings"
	"time"

	"github.com/johnewart/subcommands"

	. "github.com/yourbase/yb/packages"
	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
	. "github.com/yourbase/yb/workspace"
)

const TIME_FORMAT = "15:04:05 MST"

type BuildCmd struct {
	ExecPrefix  string
	NoContainer bool
}

type BuildConfiguration struct {
	Target           BuildTarget
	TargetDir        string
	Sandboxed        bool
	ExecPrefix       string
	ForceNoContainer bool
	TargetPackage    Package
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
	f.StringVar(&b.ExecPrefix, "exec-prefix", "", "Add a prefix to all executed commands (useful for timing or wrapping things)")
}

func (b *BuildCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	startTime := time.Now()

	fmt.Println(" === BUILD HOST ===\n")
	gi := goInfo.GetInfo()
	gi.VarDump()

	fmt.Printf("Build started at %s\n", startTime.Format(TIME_FORMAT))
	realStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputs := io.MultiWriter(realStdout)
	var buf bytes.Buffer
	uploadBuildLogs := false

	if v, err := GetConfigValue("user", "upload_build_logs"); err == nil {
		if v == "true" {
			uploadBuildLogs = true
			outputs = io.MultiWriter(realStdout, &buf)
		}
	}

	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		for {
			io.Copy(outputs, r)
			time.Sleep(100 * time.Millisecond)
		}
	}()

	defer w.Close()
	defer r.Close()

	var targetPackage Package

	// check if we're just a package
	if PathExists(MANIFEST_FILE) {
		currentPath, _ := filepath.Abs(".")
		_, pkgName := filepath.Split(currentPath)
		pkg, err := LoadPackage(pkgName, currentPath)
		if err != nil {
			fmt.Printf("Error loading package '%s': %v\n", pkgName, err)
			return subcommands.ExitFailure
		}
		targetPackage = pkg
	} else {

		workspace, err := LoadWorkspace()

		if err != nil {
			fmt.Printf("No package here, and no workspace, nothing to build!")
			return subcommands.ExitFailure
		}

		pkg, err := workspace.TargetPackage()
		if err != nil {
			fmt.Printf("Can't load workspace's target package: %v\n", err)
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

	fmt.Printf("Building target package %s in %s...\n", targetPackage.Name, targetDir)
	manifest := targetPackage.Manifest

	//workspace.SetupEnv()
	fmt.Println("\n\n === ENVIRONMENT SETUP ===\n")
	setupTimer, err := SetupBuildDependencies(targetPackage)

	if err != nil {
		fmt.Printf("Error setting up dependencies: %v\n", err)
		return subcommands.ExitFailure
	}

	var targetTimers []TargetTimer
	var buildError error

	fmt.Println("\n\n === BUILD ===\n")
	targetTimers = append(targetTimers, setupTimer)

	config := BuildConfiguration{
		ExecPrefix:       b.ExecPrefix,
		TargetDir:        targetDir,
		ForceNoContainer: b.NoContainer,
	}

	// No targets, look for default build stanza
	if len(manifest.BuildTargets) == 0 {
		target := manifest.Build
		if len(target.Commands) == 0 {
			buildError = fmt.Errorf("Default build command has no steps and no targets described\n")
		} else {
			fmt.Printf("Building target %s in %s...\n", buildTargetName, targetDir)
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
			fmt.Printf("Valid build targets: %s\n", strings.Join(manifest.BuildTargetList(), ", "))
			return subcommands.ExitFailure
		}

		fmt.Printf("Going to build targets in the following order: \n")
		for _, target := range buildTargets {
			fmt.Printf("   - %s\n", target.Name)
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

	fmt.Printf("\nBuild finished at %s, taking %s\n", endTime.Format(TIME_FORMAT), buildTime)
	fmt.Println()
	fmt.Printf("%15s%15s%15s%24s   %s\n", "Start", "End", "Elapsed", "Target", "Command")
	for _, timer := range targetTimers {
		for _, step := range timer.Timers {
			elapsed := step.EndTime.Sub(step.StartTime)
			fmt.Printf("%15s%15s%15s%24s   %s\n",
				step.StartTime.Format(TIME_FORMAT),
				step.EndTime.Format(TIME_FORMAT),
				elapsed,
				timer.Name,
				step.Command)
		}
	}
	fmt.Printf("\n%15s%15s%15s   %s\n", "", "", buildTime, "TOTAL")

	if buildError != nil {
		fmt.Println("\n\n -- BUILD FAILED -- ")
		fmt.Printf("\nBuild terminated with the following error: %v\n", buildError)
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
	targetDir := config.TargetDir
	targetPackage := config.TargetPackage

	fmt.Printf("\n\n -- Build target: %s -- \n\n", target.Name)

	// Set any environment variables as the last thing (override things we do in case people really want to do this)
	// XXX: Should we though?
	// XXX: In a perfect world we should force sandboxing by resetting all environment variables
	// XXX: Saving old state and resetting it after for now
	fmt.Printf("Setting target environment variables...\n")
	oldEnvironment := make(map[string]string)
	for _, envString := range target.Environment {
		parts := strings.Split(envString, "=")
		key := parts[0]
		value := parts[1]
		oldEnvironment[key] = os.Getenv(key)
		value = strings.Replace(value, "{PKGDIR}", targetDir, -1)
		fmt.Printf("  - %s = %s\n", key, value)
		os.Setenv(key, value)
	}

	var stepTimers []CommandTimer
	var buildError error

	if target.Container.Image != "" && !config.ForceNoContainer {
		fmt.Println("Executing build steps in container")
		containerOpts := BuildContainerOpts{
			ContainerOpts: target.Container,
			Package:       targetPackage,
		}

		stepTimers, buildError = RunCommandsInContainer(config, containerOpts)
	} else {
		// Do the commands
		fmt.Println("Executing build steps...\n")
		stepTimers, buildError = RunCommands(config)
	}

	fmt.Printf("\nResetting target environment variables...\n")
	for _, envString := range target.Environment {
		parts := strings.Split(envString, "=")
		key := parts[0]
		value := oldEnvironment[key]
		fmt.Printf("  - %s = %s\n", key, value)
		os.Setenv(key, value)
	}

	return stepTimers, buildError

}

func RunCommandsInContainer(config BuildConfiguration, containerOpts BuildContainerOpts) ([]CommandTimer, error) {
	stepTimes := make([]CommandTimer, 0)
	target := config.Target

	// Perform build inside a container
	image := target.Container.Image
	fmt.Printf("Invoking build in a container: %s\n", image)

	var buildContainer BuildContainer

	existing, err := FindContainer(containerOpts)

	if err != nil {
		fmt.Printf("Failed trying to find container: %v\n", err)
		return stepTimes, err
	}

	if existing != nil {
		fmt.Printf("Found existing container %s, removing...\n", existing.Id)
		if err = RemoveContainerById(existing.Id); err != nil {
			fmt.Printf("Unable to remove existing container: %v\n", err)
			return stepTimes, err
		}
	}

	buildContainer, err = NewContainer(containerOpts)
	if err != nil {
		fmt.Printf("Error creating build container: %v\n", err)
		return stepTimes, err
	}

	if err := buildContainer.Start(); err != nil {
		fmt.Printf("Unable to start container %s: %v", buildContainer.Id, err)
		return stepTimes, err
	}

	fmt.Printf("Building in container: %s\n", buildContainer.Id)

	for _, cmdString := range target.Commands {
		stepStartTime := time.Now()
		if len(config.ExecPrefix) > 0 {
			cmdString = fmt.Sprintf("%s %s", config.ExecPrefix, cmdString)
		}

		fmt.Printf("Running %s in the container\n", cmdString)

		if err := buildContainer.ExecToStdout(cmdString); err != nil {
			fmt.Printf("Failed to run %s: %v", cmdString, err)
			return stepTimes, fmt.Errorf("Aborting build, unable to run %s: %v\n")
		}

		stepEndTime := time.Now()
		stepTotalTime := stepEndTime.Sub(stepStartTime)

		fmt.Printf("Completed '%s' in %s\n", cmdString, stepTotalTime)

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
			//fmt.Printf("Chdir to %s\n", dir)
			//os.Chdir(dir)
			targetDir = dir
		} else {
			if target.Root != "" {
				fmt.Printf("Build root is %s\n", target.Root)
				targetDir = filepath.Join(targetDir, target.Root)
			}

			if sandboxed {
				fmt.Println("Running build in a sandbox!")
				if err := ExecInSandbox(cmdString, targetDir); err != nil {
					fmt.Printf("Failed to run %s: %v", cmdString, err)
					stepError = err
				}
			} else {
				if err := ExecToStdout(cmdString, targetDir); err != nil {
					fmt.Printf("Failed to run %s: %v", cmdString, err)
					stepError = err
				}
			}
		}

		stepEndTime := time.Now()
		stepTotalTime := stepEndTime.Sub(stepStartTime)

		fmt.Printf("Completed '%s' in %s\n", cmdString, stepTotalTime)

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
		fmt.Printf("Couldn't upload logs: %v\n", err)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Couldn't read response body: %s\n", err)
		return
	}

	if resp.StatusCode != 200 {
		fmt.Printf("Status code uploading log: %d\n", resp.StatusCode)
		fmt.Println(string(body))
		return
	} else {
		var buildLog BuildLog
		err = json.Unmarshal(body, &buildLog)
		if err != nil {
			fmt.Printf("Failed to parse response: %v\n", err)
			return
		}

		logViewPath := fmt.Sprintf("/buildlogs/%s", buildLog.UUID)
		fmt.Printf("View your build log here: %s\n", ManagementUrl(logViewPath))
	}

}

func SetupBuildDependencies(pkg Package) (TargetTimer, error) {

	startTime := time.Now()
	// Ensure build deps are :+1:
	stepTimers, err := pkg.SetupBuildDependencies()

	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)

	fmt.Printf("\nDependency setup happened in %s\n\n", elapsedTime)

	setupTimer := TargetTimer{
		Name:   "dependency_setup",
		Timers: stepTimers,
	}

	return setupTimer, err
}
