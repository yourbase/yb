package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/johnewart/subcommands"

	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

func (bc *BuildContext) doExec() (bool, error) {
	log.Printf("Execing in context id: %s\n", bc.Id)
	containerName := fmt.Sprintf("machinist-%s", bc.Id)
	instructions := bc.Instructions
	dockerClient := bc.DockerClient
	ctx := context.Background()

	imageName := instructions.Exec.Image

	log.Printf("Pulling image '%s' if needed...\n", imageName)

	reader, err := dockerClient.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, reader)

	log.Printf("Creating runtime container '%s' using '%s'\n", containerName, instructions.Exec.Image)
	var mounts = make([]mount.Mount, 0)

	err = os.Mkdir("build", 0700)
	if err != nil {
		log.Printf("build directory already exists!\n")
	}

	buildPath, err := filepath.Abs("build")

	if err != nil {
		return false, fmt.Errorf("Couldn't get absolute path to build directory!")
	}
	mounts = append(mounts, mount.Mount{
		Source: buildPath,
		Target: "/workspace/",
		Type:   mount.TypeBind,
	})

	var networkConfig = &network.NetworkingConfig{}
	var hostConfig = &container.HostConfig{
		Mounts: mounts,
	}
	var containerConfig = &container.Config{
		Image:        imageName,
		Cmd:          strings.Split(instructions.Exec.Command, " "),
		WorkingDir:   "/workspace/",
		Tty:          true,
		AttachStdout: true,
	}

	buildContainer, err := dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, containerName)
	if err != nil {
		panic(err)
	}

	c := make(chan os.Signal, 1)

	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			log.Printf("Signal: %s", sig)
			// sig is a ^C, handle it
			log.Printf("CTRL-C hit, cleaning up!")
			// Terminate now, not in a bit
			stopTime, _ := time.ParseDuration("-1m")

			err := dockerClient.ContainerStop(ctx, buildContainer.ID, &stopTime)

			if err != nil {
				log.Printf("Unable to stop container...")
			}
			bc.Cleanup()
		}
	}()

	log.Printf("Going to run '%s'...\n", instructions.Exec.Command)

	log.Printf("Starting exec container...\n")
	if err := dockerClient.ContainerStart(ctx, buildContainer.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	props, err := dockerClient.ContainerInspect(ctx, buildContainer.ID)

	if err != nil {
		panic(err)
	}

	networkSettings := props.NetworkSettings
	ipv4 := networkSettings.DefaultNetworkSettings.IPAddress
	log.Printf("Container IP: %s\n", ipv4)

	for _, port := range instructions.Exec.Ports {
		log.Printf("   %s\n", port)
	}

	out, err := dockerClient.ContainerLogs(ctx, buildContainer.ID, types.ContainerLogsOptions{
		ShowStderr: true,
		ShowStdout: true,
		Timestamps: false,
		Follow:     true,
		Tail:       "40",
	})
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			io.Copy(os.Stdout, out)
		}
	}()

	log.Printf("Waiting for exec to complete...")
	statusCh, errCh := dockerClient.ContainerWait(ctx, buildContainer.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			panic(err)
		}
	case <-statusCh:
	}

	return true, nil
}

type execCmd struct {
	capitalize bool
}

func (*execCmd) Name() string     { return "exec" }
func (*execCmd) Synopsis() string { return "Execute the target project in the workspace" }
func (*execCmd) Usage() string {
	return `exec`
}

func (p *execCmd) SetFlags(f *flag.FlagSet) {
}

/*
Executing the target involves:
1. Map source into the target container
2. Run any dependent components
3. Start target
*/
func (b *execCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	workspace := LoadWorkspace()
	targetPackage := workspace.Target

	log.Printf("Execing target package %s...\n", targetPackage)
	err := os.Chdir(targetPackage)
	if err != nil {
		return subcommands.ExitFailure
	}
	defer os.Chdir("..")

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.39"))
	if err != nil {
		panic(err)
	}

	pwd, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		fmt.Errorf("Can't get PWD!\n")
	}
	ctx, err := NewContext(dockerClient, pwd)
	if err != nil {
		return subcommands.ExitFailure
	}

	defer ctx.Cleanup()
	//ctx.startDependencies()
	ctx.doExec()

	return subcommands.ExitSuccess
}
