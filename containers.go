package main

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/nu7hatch/gouuid"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ServiceContext struct {
	DockerClient *client.Client
	Id           string
	Containers   []ContainerDefinition
	PackageName  string
	NetworkId    string
}

func NewServiceContext(packageName string, containers []ContainerDefinition) (*ServiceContext, error) {
	ctxId, err := uuid.NewV4()
	ctx := context.Background()

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.39"))
	if err != nil {
		panic(err)
	}

	log.Printf("Creating network %s...\n", ctxId.String())
	opts := types.NetworkCreate{}
	response, err := dockerClient.NetworkCreate(ctx, ctxId.String(), opts)
	if err != nil {
		return nil, err
	}

	sc := &ServiceContext{
		PackageName:  packageName,
		Id:           ctxId.String(),
		DockerClient: dockerClient,
		Containers:   containers,
		NetworkId:    response.ID,
	}

	return sc, nil
}

func (sc *ServiceContext) SetupNetwork() (string, error) {
	ctx := context.Background()
	dockerClient := sc.DockerClient
	opts := types.NetworkCreate{}

	log.Printf("Creating network %s...\n", sc.Id)
	response, err := dockerClient.NetworkCreate(ctx, sc.Id, opts)

	if err != nil {
		return "", err
	}

	return response.ID, nil
}

func (sc *ServiceContext) Cleanup() error {
	ctx := context.Background()
	dockerClient := sc.DockerClient

	log.Printf("Removing network %s\n", sc.Id)
	err := dockerClient.NetworkRemove(ctx, sc.Id)

	if err != nil {
		return err
	}

	return nil

}

func (sc *ServiceContext) StandUp() error {
	workspace := LoadWorkspace()
	buildRoot := workspace.BuildRoot()
	pkgWorkdir := filepath.Join(buildRoot, sc.PackageName)
	MkdirAsNeeded(pkgWorkdir)
	logDir := filepath.Join(pkgWorkdir, "logs")
	MkdirAsNeeded(logDir)

	for _, c := range sc.Containers {
		fmt.Printf("  %s...\n", c.Image)

		s := strings.Split(c.Image, ":")
		imageName := s[0]

		containerName := fmt.Sprintf("%s-%s", sc.PackageName, imageName)

		dockerClient := sc.DockerClient
		ctx := context.Background()

		filters := filters.NewArgs()
		filters.Add("name", containerName)
		containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{
			Size:    true,
			All:     true,
			Filters: filters,
		})

		if len(containers) > 0 {
			fmt.Printf("Container '%s' already exists, not creating...\n", containerName)
			continue
		}

		fmt.Printf("Pulling image '%s' if needed...\n", c.Image)

		reader, err := dockerClient.ImagePull(ctx, c.Image, types.ImagePullOptions{})
		if err != nil {
			panic(err)
		}
		io.Copy(os.Stdout, reader)

		var mounts = make([]mount.Mount, 0)

		for _, mountSpec := range c.Mounts {
			s := strings.Split(mountSpec, ":")
			src := filepath.Join(pkgWorkdir, s[0])

			MkdirAsNeeded(src)

			dst := s[1]
			mounts = append(mounts, mount.Mount{
				Source: src,
				Target: dst,
				Type:   mount.TypeBind,
			})
		}

		_, portmap, err := nat.ParsePortSpecs(c.Ports)

		/*portmap := nat.PortMap{
			nat.Port(fmt.Sprintf("%d/tcp", port)): []nat.PortBinding{
				{
					HostPort: fmt.Sprintf("%d", port),
				},
			},
		}*/

		if err != nil {
			fmt.Printf("Unable to parse ports: %v\n", err)
			return err
		}

		// Environment variables for the build tools' cache
		env := make([]string, 0)

		for _, e := range c.Environment {
			env = append(env, e)
		}

		var networkConfig = &network.NetworkingConfig{}
		var hostConfig = &container.HostConfig{
			Mounts:       mounts,
			PortBindings: portmap,
		}

		var containerConfig = &container.Config{
			Image:        c.Image,
			Tty:          false,
			AttachStdout: false,
			Env:          env,
		}

		dependencyContainer, err := dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, containerName)
		if err != nil {
			panic(err)
		}

		log.Printf("Connecting to network %s...\n", sc.NetworkId)
		if err := dockerClient.NetworkConnect(ctx, sc.NetworkId, dependencyContainer.ID, &network.EndpointSettings{}); err != nil {
			panic(err)
		}

		log.Printf("Starting container...\n")
		if err := dockerClient.ContainerStart(ctx, dependencyContainer.ID, types.ContainerStartOptions{}); err != nil {
			panic(err)
		}

		props, err := dockerClient.ContainerInspect(ctx, dependencyContainer.ID)

		if err != nil {
			panic(err)
		}

		networkSettings := props.NetworkSettings
		ipv4 := networkSettings.DefaultNetworkSettings.IPAddress
		log.Printf("Container IP: %s\n", ipv4)

		//TODO: stream logs from each dependency to the build dir
		containerLogFile := filepath.Join(logDir, fmt.Sprintf("%s.log", imageName))
		f, err := os.Create(containerLogFile)

		if err != nil {
			fmt.Printf("Unable to write to log file %s: %v\n", containerLogFile, err)
			return err
		}

		out, err := dockerClient.ContainerLogs(ctx, dependencyContainer.ID, types.ContainerLogsOptions{
			ShowStderr: true,
			ShowStdout: true,
			Timestamps: false,
			Follow:     true,
			Tail:       "40",
		})
		if err != nil {
			fmt.Printf("Can't get log handle for container %s: %v\n", dependencyContainer.ID, err)
			return err
		}
		go func() {
			for {
				io.Copy(f, out)
				time.Sleep(300 * time.Millisecond)
			}
		}()
	}

	return nil
}
