package main

import (
	"archive/tar"
	"context"
	"flag"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/google/subcommands"

	"github.com/nu7hatch/gouuid"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type BuildContext struct {
	DockerClient *client.Client
	Id           *uuid.UUID
	Instructions BuildInstructions
}

type BuildInstructions struct {
	Build     BuildPhase     `yaml:"build"`
	Container ContainerPhase `yaml:"container"`
	Exec      ExecPhase      `yaml:"exec"`
}

type ExecPhase struct {
	Image   string `yaml:"image"`
	Command string `yaml:"command"`
}

type BuildPhase struct {
	Image     string   `yaml:"image"`
	Command   string   `yaml:"command"`
	Sandbox   bool     `yaml:"sandbox"`
	Artifacts []string `yaml:"artifacts"`
}

type ContainerPhase struct {
	BaseImage string   `yaml:"base_image"`
	Command   string   `yaml:"command"`
	Artifacts []string `yaml:"artifacts"`
}

func archiveSource(source, target string) error {
	return archiveDirectory(source, target, "workspace")
}

func archiveArtifacts(source, target string) error {
	return archiveDirectory(source, target, "")
}

func archiveDirectory(source, target, prefix string) error {
	tarfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer tarfile.Close()

	tarball := tar.NewWriter(tarfile)
	defer tarball.Close()

	info, err := os.Stat(source)
	if err != nil {
		return nil
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
	}
	fmt.Printf("Base dir: %s\n", baseDir)

	return filepath.Walk(source,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			fmt.Printf("Adding %s ...\n", info.Name())
			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}

			if prefix != "" {
				header.Name = filepath.Join(prefix, strings.TrimPrefix(path, source))
			} else {
				header.Name = filepath.Join(".", strings.TrimPrefix(path, source))
			}

			if err := tarball.WriteHeader(header); err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(tarball, file)
			return err
		})

}

func listImages() {
	/*
		imgs, _ := dockerClient.ListImages(docker.ListImagesOptions{All: true})
		for _, img := range imgs {
			fmt.Println("ID: ", img.ID)
			fmt.Println("RepoTags: ", img.RepoTags)
			fmt.Println("Created: ", img.Created)
			fmt.Println("Size: ", img.Size)
			fmt.Println("VirtualSize: ", img.VirtualSize)
			fmt.Println("ParentId: ", img.ParentID)
		}
	*/
	return
}

func (bc *BuildContext) doBuild() (bool, error) {
	fmt.Printf("Building in context id: %s\n", bc.Id)
	containerName := fmt.Sprintf("machinist-%s", bc.Id)
	instructions := bc.Instructions
	dockerClient := bc.DockerClient
	ctx := context.Background()

	imageName := instructions.Build.Image

	fmt.Printf("Pulling image '%s' if needed...\n", imageName)

	reader, err := dockerClient.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, reader)

	fmt.Printf("Creating build container '%s' using '%s'\n", containerName, instructions.Build.Image)
	var networkConfig = &network.NetworkingConfig{}
	var hostConfig = &container.HostConfig{}
	var containerConfig = &container.Config{
		Image:        imageName,
		Cmd:          strings.Split(instructions.Build.Command, " "),
		Tty:          true,
		AttachStdout: true,
		WorkingDir:   "/workspace",
	}

	buildContainer, err := dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, containerName)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Going to run '%s'...\n", instructions.Build.Command)

	dir, err := ioutil.TempDir("", "machinist-src")
	if err != nil {
		log.Fatal(err)
	}

	//defer os.RemoveAll(dir) // clean up
	tmpfile, err := os.OpenFile(fmt.Sprintf("%s/src.tar", dir), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0660)
	if err != nil {
		log.Fatal(err)
	}

	//defer os.Remove(tmpfile.Name())
	fmt.Printf("Archiving source to %s...\n", tmpfile.Name())
	archiveSource("src", tmpfile.Name())
	fmt.Printf("Done!\n")
	tarStream, err := os.Open(tmpfile.Name())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Adding source to container...\n")
	if err := dockerClient.CopyToContainer(ctx, buildContainer.ID, "/", tarStream, types.CopyToContainerOptions{}); err != nil {
		panic(err)
	}

	fmt.Printf("Starting build container...\n")
	if err := dockerClient.ContainerStart(ctx, buildContainer.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	fmt.Printf("Waiting for build to complete...")
	statusCh, errCh := dockerClient.ContainerWait(ctx, buildContainer.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			panic(err)
		}
	case <-statusCh:
	}

	out, err := dockerClient.ContainerLogs(ctx, buildContainer.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		panic(err)
	}

	io.Copy(os.Stdout, out)

	dir, err = ioutil.TempDir("", "machinist-artifacts")

	for _, element := range instructions.Build.Artifacts {
		downloadPath := fmt.Sprintf("/workspace/%s", element)
		fileParts := strings.Split(element, "/")
		filename := fileParts[len(fileParts)-1]
		outfileName := fmt.Sprintf("%s/%s", dir, filename)

		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Extracting %s to %s...\n", element, outfileName)

		stream, _, err := dockerClient.CopyFromContainer(ctx, buildContainer.ID, downloadPath)
		outFile, err := os.Create(outfileName)
		// handle err
		defer outFile.Close()
		_, err = io.Copy(outFile, stream)
		// handle err

		if err != nil {
			log.Fatal(err)
		}

	}

	archiveArtifacts(dir, "artifacts.tar")

	return true, nil
}

func NewContext(dockerClient *client.Client, id *uuid.UUID, projectDir string) BuildContext {
	ctx := BuildContext{}
	ctx.DockerClient = dockerClient
	ctx.Id = id
	ctx.Instructions = BuildInstructions{}

	buildyaml, _ := ioutil.ReadFile("build.yml")
	err := yaml.Unmarshal([]byte(buildyaml), &ctx.Instructions)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	fmt.Printf("--- i:\n%v\n\n", ctx.Instructions)

	return ctx
}

type buildCmd struct {
	capitalize bool
}

func (*buildCmd) Name() string     { return "build" }
func (*buildCmd) Synopsis() string { return "Build the workspace" }
func (*buildCmd) Usage() string {
	return `build [-capitalize]`
}

func (p *buildCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&p.capitalize, "capitalize", false, "capitalize output")
}

func (b *buildCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.39"))
	if err != nil {
		panic(err)
	}

	containers, err := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	for _, container := range containers {
		fmt.Printf("%s %s\n", container.ID[:10], container.Image)
	}
	ctxId, err := uuid.NewV4()
	pwd, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		fmt.Errorf("Can't get PWD!\n")
	}
	ctx := NewContext(dockerClient, ctxId, pwd)
	ctx.doBuild()

	return subcommands.ExitSuccess
}
