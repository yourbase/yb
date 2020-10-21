package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/yourbase/narwhal"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/build"
	"github.com/yourbase/yb/packages"
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

func newBiome(ctx context.Context, client *docker.Client, packageDir string) (biome.Biome, error) {
	// TODO(ch2743): Eventually also allow Docker container.
	return biome.Local{
		PackageDir: packageDir,
	}, nil
}

func targetToPhaseDeps(target *types.BuildTarget) (*build.PhaseDeps, error) {
	phaseDeps := &build.PhaseDeps{
		TargetName: target.Name,
		Resources:  narwhalContainerMap(target.Dependencies.Containers),
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

func GetTargetPackageNamed(file string) (*packages.Package, error) {
	if _, err := os.Stat(file); err != nil {
		return nil, fmt.Errorf("could not find configuration: %w\n\nTry running in the package root dir or writing the YML config file (%s) if it is missing. See %s", err, file, types.DOCS_URL)
	}
	currentPath, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not find configuration: %w", err)
	}
	pkgName := filepath.Base(currentPath)
	pkg, err := packages.LoadPackage(pkgName, currentPath)
	if err != nil {
		return nil, fmt.Errorf("loading package %s: %w\n\nSee %s\n", pkgName, err, types.DOCS_URL)
	}
	return pkg, nil
}

func GetTargetPackage() (*packages.Package, error) {
	return GetTargetPackageNamed(types.MANIFEST_FILE)
}
