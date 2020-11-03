package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/yourbase/narwhal"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/build"
	"github.com/yourbase/yb/internal/ybdata"
	"github.com/yourbase/yb/types"
	"zombiezen.com/go/log"
)

func connectDockerClient(useDocker bool) (*docker.Client, error) {
	if !useDocker {
		return nil, nil
	}
	dockerClient, err := docker.NewVersionedClient("unix:///var/run/docker.sock", "1.39")
	if err != nil {
		return nil, err
	}
	return dockerClient, nil
}

const netrcFilename = ".netrc"

func newBiome(ctx context.Context, client *docker.Client, dataDirs *ybdata.Dirs, packageDir, target string) (biome.BiomeCloser, error) {
	// TODO(ch2743): Eventually also allow Docker container.
	l := biome.Local{
		PackageDir: packageDir,
	}
	var err error
	l.HomeDir, err = dataDirs.BuildHome(packageDir, target, l.Describe())
	if err != nil {
		return nil, fmt.Errorf("create build context for target %s: %w", target, err)
	}
	bio, err := injectNetrc(ctx, l)
	if err != nil {
		return nil, fmt.Errorf("create build context for target %s: %w", target, err)
	}
	return bio, nil
}

func injectNetrc(ctx context.Context, bio biome.BiomeCloser) (biome.BiomeCloser, error) {
	const gitHubTokenVar = "YB_GH_TOKEN"
	token := os.Getenv(gitHubTokenVar)
	if token == "" {
		return bio, nil
	}
	log.Infof(ctx, "Writing .netrc")
	netrcPath := bio.JoinPath(bio.Dirs().Home, netrcFilename)
	err := biome.WriteFile(ctx, bio, netrcPath, bytes.NewReader(generateNetrc(token)))
	if err != nil {
		return nil, fmt.Errorf("write netrc: %w", err)
	}
	err = runCommand(ctx, bio, "chmod", "600", netrcPath)
	if err != nil {
		// Not fatal. File will be removed later.
		log.Warnf(ctx, "Making temporary .netrc private: %v", err)
	}
	bio = biome.WithClose(bio, func() error {
		ctx := context.Background()
		err := runCommand(ctx, bio,
			"rm", bio.JoinPath(bio.Dirs().Home, netrcFilename))
		if err != nil {
			log.Warnf(ctx, "Could not clean up .netrc: %v", err)
		}
		return nil
	})
	return biome.EnvBiome{
		Biome: bio,
		Env: biome.Environment{
			Vars: map[string]string{
				gitHubTokenVar: token,
			},
		},
	}, nil
}

func generateNetrc(gitHubToken string) []byte {
	if gitHubToken == "" {
		return nil
	}
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "machine github.com login x-access-token password %s\n", gitHubToken)
	return buf.Bytes()
}

func runCommand(ctx context.Context, bio biome.Biome, argv ...string) error {
	output := new(strings.Builder)
	err := bio.Run(ctx, &biome.Invocation{
		Argv:   argv,
		Stdout: output,
		Stderr: output,
	})
	if err != nil {
		if output.Len() > 0 {
			return fmt.Errorf("%w\n%s", err, output)
		}
		return err
	}
	return nil
}

func targetToPhaseDeps(target *types.BuildTarget) (*build.PhaseDeps, error) {
	phaseDeps := &build.PhaseDeps{
		TargetName: target.Name,
		Resources:  narwhalContainerMap(target.Dependencies.Containers),
	}
	for _, dep := range target.Dependencies.Build {
		spec, err := types.ParseBuildpackSpec(dep)
		if err != nil {
			return nil, fmt.Errorf("target %s: %w", target.Name, err)
		}
		phaseDeps.Buildpacks = append(phaseDeps.Buildpacks, spec)
	}
	var err error
	phaseDeps.EnvironmentTemplate, err = biome.MapVars(target.Environment)
	if err != nil {
		return nil, fmt.Errorf("target %s: %w", target.Name, err)
	}
	return phaseDeps, nil
}

func narwhalContainerMap(defs map[string]*types.ContainerDefinition) map[string]*narwhal.ContainerDefinition {
	if len(defs) == 0 {
		return nil
	}
	nmap := make(map[string]*narwhal.ContainerDefinition, len(defs))
	for k, cd := range defs {
		nmap[k] = cd.ToNarwhal()
	}
	return nmap
}

func targetToPhase(target *types.BuildTarget) *build.Phase {
	return &build.Phase{
		TargetName: target.Name,
		Commands:   target.Commands,
		Root:       target.Root,
	}
}

func newDockerNetwork(ctx context.Context, client *docker.Client) (string, func(), error) {
	if client == nil {
		return "", func() {}, nil
	}
	var bits [8]byte
	if _, err := rand.Read(bits[:]); err != nil {
		return "", nil, fmt.Errorf("create docker network: generate name: %w", err)
	}
	name := hex.EncodeToString(bits[:])
	log.Infof(ctx, "Creating Docker network %s...", name)
	network, err := client.CreateNetwork(docker.CreateNetworkOptions{
		Context: ctx,
		Name:    name,
		Driver:  "bridge",
	})
	if err != nil {
		return "", nil, fmt.Errorf("create docker network: %w", err)
	}
	id := network.ID
	return id, func() {
		log.Infof(ctx, "Removing Docker network %s...", name)
		if err := client.RemoveNetwork(id); err != nil {
			log.Warnf(ctx, "Unable to remove Docker network %s (%s): %v", name, id, err)
		}
	}, nil
}

const packageConfigFileName = ".yourbase.yml"

func GetTargetPackage() (*types.Package, error) {
	return types.LoadPackage(packageConfigFileName)
}
