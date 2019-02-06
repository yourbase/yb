package main

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"

	"github.com/nu7hatch/gouuid"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
)

type BuildContext struct {
	DockerClient *client.Client
	Id           string
	Instructions BuildInstructions
}

type BuildInstructions struct {
	Build     BuildPhase     `yaml:"build"`
	Container ContainerPhase `yaml:"container"`
	Exec      ExecPhase      `yaml:"exec"`
}

type ExecPhase struct {
	Image   string   `yaml:"image"`
	Command string   `yaml:"command"`
	Ports   []string `yaml:"ports"`
}

type BuildPhase struct {
	Image     string   `yaml:"image"`
	Commands  []string `yaml:"commands"`
	Sandbox   bool     `yaml:"sandbox"`
	Artifacts []string `yaml:"artifacts"`
}

type ContainerPhase struct {
	BaseImage string   `yaml:"base_image"`
	Command   string   `yaml:"command"`
	Artifacts []string `yaml:"artifacts"`
}

func NewContext(dockerClient *client.Client, projectDir string) (BuildContext, error) {

	ctxId, err := uuid.NewV4()

	ctx := BuildContext{}
	ctx.DockerClient = dockerClient
	ctx.Id = ctxId.String()
	ctx.Instructions = BuildInstructions{}

	if _, err := os.Stat("build.yml"); os.IsNotExist(err) {
		panic("No build.yml -- can't build!")
	}

	buildyaml, _ := ioutil.ReadFile("build.yml")
	err = yaml.Unmarshal([]byte(buildyaml), &ctx.Instructions)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	fmt.Printf("--- i:\n%v\n\n", ctx.Instructions)

	_, err = ctx.SetupNetwork()

	if err != nil {
		return ctx, err
	}

	return ctx, nil
}

func (bc *BuildContext) Cleanup() error {
	ctx := context.Background()
	dockerClient := bc.DockerClient

	log.Printf("Removing network %s\n", bc.Id)
	err := dockerClient.NetworkRemove(ctx, bc.Id)

	if err != nil {
		return err
	}

	return nil

}

func (bc *BuildContext) SetupNetwork() (string, error) {
	ctx := context.Background()
	dockerClient := bc.DockerClient
	opts := types.NetworkCreate{}

	log.Printf("Creating network %s...\n", bc.Id)
	response, err := dockerClient.NetworkCreate(ctx, bc.Id, opts)

	if err != nil {
		return "", err
	}

	return response.ID, nil
}
