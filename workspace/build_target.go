package workspace

import (
	"context"
	"fmt"
	"github.com/yourbase/narwhal"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
	"io"
	"os"
	"strings"
	"time"
)

type CommandTimer struct {
	Command   string
	StartTime time.Time
	EndTime   time.Time
}

type TargetTimer struct {
	Name   string
	Timers []CommandTimer
}

type BuildTarget struct {
	Name         string                      `yaml:"name"`
	Container    narwhal.ContainerDefinition `yaml:"container"`
	Tools        []string                    `yaml:"tools"`
	Commands     []string                    `yaml:"commands"`
	Artifacts    []string                    `yaml:"artifacts"`
	CachePaths   []string                    `yaml:"cache_paths"`
	Sandbox      bool                        `yaml:"sandbox"`
	HostOnly     bool                        `yaml:"host_only"`
	Root         string                      `yaml:"root"`
	Environment  []string                    `yaml:"environment"`
	Tags         map[string]string           `yaml:"tags"`
	BuildAfter   []string                    `yaml:"build_after"`
	Dependencies BuildDependencies           `yaml:"dependencies"`
}

type BuildDependencies struct {
	Containers map[string]narwhal.ContainerDefinition `yaml:"containers"`
}

func (b BuildDependencies) ContainerList() []narwhal.ContainerDefinition {
	containers := make([]narwhal.ContainerDefinition, 0)
	for label, c := range b.Containers {
		c.Label = label
		containers = append(containers, c)
	}
	return containers
}

func (bt BuildTarget) EnvironmentVariables(data runtime.RuntimeEnvironmentData) []string {
	result := make([]string, 0)
	for _, property := range bt.Environment {
		if _, _, ok := plumbing.SaneEnvironmentVar(property); ok {
			interpolated, err := TemplateToString(property, data)
			if err == nil {
				result = append(result, interpolated)
			} else {
				result = append(result, property)
			}
		}
	}

	return result
}

func (bt BuildTarget) Build(ctx context.Context, runtimeCtx *runtime.Runtime, output io.Writer, flags BuildFlags, packagePath string, buildpacks []string) ([]CommandTimer, error) {
	var stepTimes []CommandTimer

	containers := bt.Dependencies.ContainerList()
	workDir := packagePath
	builder := runtimeCtx.DefaultTarget

	hostOnly := bt.HostOnly || flags.HostOnly

	if !hostOnly {
		stepStartTime := time.Now()

		buildContainer := bt.Container
		buildContainer.Command = "/usr/bin/tail -f /dev/null"
		buildContainer.Label = "build"

		// Append build environment variables
		buildContainer.Environment = []string{}

		// Add package to mounts @ /workspace
		sourceMapDir := "/workspace"
		if buildContainer.WorkDir != "" {
			sourceMapDir = buildContainer.WorkDir
		}

		log.Infof("Will mount  %s at %s in container", packagePath, sourceMapDir)
		mount := fmt.Sprintf("%s:%s", packagePath, sourceMapDir)
		buildContainer.Mounts = append(buildContainer.Mounts, mount)

		containers = append(containers, buildContainer)

		var err error
		builder, err = runtimeCtx.AddContainer(ctx, buildContainer)

		stepEndTime := time.Now()
		stepTotalTime := stepEndTime.Sub(stepStartTime)

		log.Infof("Completed container prep in %s", stepTotalTime)

		containerTimer := CommandTimer{
			Command:   "internal container prep",
			StartTime: stepStartTime,
			EndTime:   stepEndTime,
		}

		stepTimes = append(stepTimes, containerTimer)
		if err != nil {
			return stepTimes, err
		}

		stepStartTime = time.Now()

		runtimeCtx.DefaultTarget = builder
		workDir = sourceMapDir

		// Inject a .ssh/config to skip host key checking
		sshConfig := "Host github.com\n\tStrictHostKeyChecking no\n"
		builder.Run(ctx, runtime.Process{Output: output, Command: "mkdir -p /root/.ssh"})
		builder.WriteFileContents(ctx, sshConfig, "/root/.ssh/config")
		builder.Run(ctx, runtime.Process{Output: output, Command: "chmod 0600 /root/.ssh/config"})
		builder.Run(ctx, runtime.Process{Output: output, Command: "chown root:root /root/.ssh/config"})

		// Inject a useful gitconfig
		configlines := []string{
			"[url \"ssh://git@github.com/\"]",
			"insteadOf = https://github.com/",
			"[url \"ssh://git@gitlab.com/\"]",
			"insteadOf = https://gitlab.com/",
			"[url \"ssh://git@bitbucket.org/\"]",
			"insteadOf = https://bitbucket.org/",
		}
		gitConfig := strings.Join(configlines, "\n")
		builder.WriteFileContents(ctx, gitConfig, "/root/.gitconfig")

		// TODO: Don't run this multiple times
		// Map SSH agent into the container
		if agentPath, exists := os.LookupEnv("SSH_AUTH_SOCK"); exists {
			log.Infof("Running SSH agent socket forwarder...")
			hostAddr, err := runtime.ForwardUnixSocketToTcp(agentPath)
			if err != nil {
				log.Warnf("Could not forward SSH agent: %v", err)
			} else {
				log.Infof("Forwarding SSH agent via %s", hostAddr)
			}

			builder.SetEnv("SSH_AUTH_SOCK", "/ssh_agent")
			forwardPath, err := builder.DownloadFile(ctx, "https://yourbase-artifacts.s3-us-west-2.amazonaws.com/sockforward")
			builder.Run(ctx, runtime.Process{Output: output, Command: fmt.Sprintf("chmod a+x %s", forwardPath)})
			forwardCmd := fmt.Sprintf("%s /ssh_agent %s", forwardPath, hostAddr)
			go func() {
				builder.Run(ctx, runtime.Process{Output: output, Command: forwardCmd})
			}()
		}

		stepEndTime = time.Now()
		stepTotalTime = stepEndTime.Sub(stepStartTime)

		log.Infof("Completed container tweaking in %s", stepTotalTime)

		tweakTimer := CommandTimer{
			Command:   "internal container tweak",
			StartTime: stepStartTime,
			EndTime:   stepEndTime,
		}

		stepTimes = append(stepTimes, tweakTimer)
		if err != nil {
			return stepTimes, err
		}

	}

	depContainerStartTime := time.Now()
	// Setup dependent containers
	for _, cd := range bt.Dependencies.ContainerList() {
		if _, err := runtimeCtx.AddContainer(ctx, cd); err != nil {
			errorTime := time.Now()
			errorTotalTime := errorTime.Sub(depContainerStartTime)

			log.Infof("When adding container %s, took %s", cd.Label, errorTotalTime)

			errorTimer := CommandTimer{
				Command:   "Starting dependencies (containers)",
				StartTime: depContainerStartTime,
				EndTime:   errorTime,
			}
			stepTimes = append(stepTimes, errorTimer)

			return stepTimes, fmt.Errorf("adding container %s: %v", cd.Label, err)
		}
	}

	// Do this after the containers are up
	for _, envString := range bt.EnvironmentVariables(runtimeCtx.EnvironmentData()) {
		if n, v, ok := plumbing.SaneEnvironmentVar(envString); ok {
			builder.SetEnv(n, v)
		} else {
			log.Warnf("'%s' doesn't look like an environment variable", envString)
		}
	}

	buildPackStartTime := time.Now()
	buildPackTimes, err := LoadBuildPacks(ctx, builder, buildpacks)
	buildPacksTotalTime := time.Since(buildPackStartTime)

	log.Infof("Completed loading build packs in: %s", buildPacksTotalTime)
	stepTimes = append(stepTimes, buildPackTimes...)

	if err != nil {
		log.Errorf("Build packs: %v", err)
		return stepTimes, err
	}

	if flags.DependenciesOnly {
		return stepTimes, nil
	}

	for _, cmdString := range bt.Commands {
		var stepError error

		if flags.ExecPrefix != "" {
			cmdString = flags.ExecPrefix + " " + cmdString
		}

		stepStartTime := time.Now()
		p := runtime.Process{
			Directory: workDir,
			Command:   cmdString,
			//Environment: buildData.environmentVariables(),
			Interactive: false,
		}

		if stepError = builder.Run(ctx, p); stepError != nil {
			log.Errorf("Failed to run %s: %v", cmdString, stepError)
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
		if stepError != nil {
			return stepTimes, stepError
		}
	}

	return stepTimes, nil
}
