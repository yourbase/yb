package main

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"

	"github.com/nu7hatch/gouuid"
	"log"
)

type BuildContext struct {
	DockerClient *client.Client
	Id           string
	Instructions BuildInstructions
}

type BuildInstructions struct {
	Dependencies DependencySet `yaml:"dependencies"`
	Build        BuildPhase    `yaml:"build"`
	Exec         ExecPhase     `yaml:"exec"`
}

type DependencySet struct {
	Build   []string `yaml:"build"`
	Runtime []string `yaml:"runtime"`
}

type ExecPhase struct {
	Image    string   `yaml:"image"`
	Commands []string `yaml:"commands"`
	Ports    []string `yaml:"ports"`
}

type BuildPhase struct {
	Tools       []string `yaml:"tools"`
	Commands    []string `yaml:"commands"`
	Sandbox     bool     `yaml:"sandbox"`
	Artifacts   []string `yaml:"artifacts"`
	Environment []string `yaml:"env"`
}

func NewContext() (BuildContext, error) {

	ctxId, err := uuid.NewV4()

	ctx := BuildContext{}

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.39"))
	if err != nil {
		panic(err)
	}

	ctx.DockerClient = dockerClient
	ctx.Id = ctxId.String()
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
