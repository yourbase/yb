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
	Manifest     BuildManifest
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
