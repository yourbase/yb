package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/yourbase/narwhal"
	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/config"
	"github.com/yourbase/yb/internal/ybdata"
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

type newBiomeOptions struct {
	packageDir string
	target     string
	downloader *ybdata.Downloader
	dataDirs   *ybdata.Dirs
	baseEnv    biome.Environment
	netrcFiles []string

	dockerClient    *docker.Client
	targetContainer *narwhal.ContainerDefinition
	dockerNetworkID string
}

func (opts newBiomeOptions) disableDocker() newBiomeOptions {
	// Operating on copy, so free to modify fields.
	opts.dockerClient = nil
	opts.targetContainer = nil
	opts.dockerNetworkID = ""
	return opts
}

func newBiome(ctx context.Context, opts newBiomeOptions) (biome.BiomeCloser, error) {
	log.Debugf(ctx, "Checking for netrc data in %s",
		append(append([]string(nil), config.DefaultNetrcFiles()...), opts.netrcFiles...))
	netrc, err := config.CatFiles(config.DefaultNetrcFiles(), opts.netrcFiles)
	if err != nil {
		return nil, fmt.Errorf("set up environment for target %s: %w", opts.target, err)
	}
	if opts.dockerClient == nil {
		l := biome.Local{
			PackageDir: opts.packageDir,
		}
		var err error
		l.HomeDir, err = opts.dataDirs.BuildHome(opts.packageDir, opts.target, l.Describe())
		if err != nil {
			return nil, fmt.Errorf("set up environment for target %s: %w", opts.target, err)
		}
		log.Debugf(ctx, "Home located at %s", l.HomeDir)
		bio, err := injectNetrc(ctx, l, netrc)
		if err != nil {
			return nil, fmt.Errorf("set up environment for target %s: %w", opts.target, err)
		}
		return biome.EnvBiome{
			Biome: bio,
			Env:   opts.baseEnv,
		}, nil
	}

	home, err := opts.dataDirs.BuildHome(opts.packageDir, opts.target, biome.DockerDescriptor())
	if err != nil {
		return nil, fmt.Errorf("set up environment for target %s: %w", opts.target, err)
	}
	log.Debugf(ctx, "Home located at %s", home)
	tiniFile, err := opts.downloader.Download(ctx, biome.TiniURL)
	if err != nil {
		return nil, fmt.Errorf("set up environment for target %s: %w", opts.target, err)
	}
	defer tiniFile.Close()
	c, err := biome.CreateContainer(ctx, opts.dockerClient, &biome.ContainerOptions{
		PackageDir: opts.packageDir,
		HomeDir:    home,
		TiniExe:    tiniFile,
		Definition: opts.targetContainer,
		NetworkID:  opts.dockerNetworkID,
		PullOutput: os.Stderr,
	})
	if err != nil {
		return nil, fmt.Errorf("set up environment for target %s: %w", opts.target, err)
	}
	bio, err := injectNetrc(ctx, c, netrc)
	if err != nil {
		return nil, fmt.Errorf("set up environment for target %s: %w", opts.target, err)
	}
	return biome.EnvBiome{
		Biome: bio,
		Env:   opts.baseEnv,
	}, nil
}

// netrcFlagVar registers the --netrc flag.
func netrcFlagVar(flags *pflag.FlagSet, netrc *[]string) {
	// StringArray makes every --netrc flag add to the list.
	// StringSlice does this too, but also permits comma-separated.
	// (Not great names. It isn't obvious until you look at the source.)
	flags.StringArrayVar(netrc, "netrc-file", nil, "Inject a netrc `file` (can be passed multiple times to concatenate)")
}

func injectNetrc(ctx context.Context, bio biome.BiomeCloser, netrc []byte) (biome.BiomeCloser, error) {
	if len(netrc) == 0 {
		log.Debugf(ctx, "No .netrc data, skipping")
		return bio, nil
	}
	const netrcFilename = ".netrc"
	log.Infof(ctx, "Writing .netrc")
	netrcPath := bio.JoinPath(bio.Dirs().Home, netrcFilename)
	err := biome.WriteFile(ctx, bio, netrcPath, bytes.NewReader(netrc))
	if err != nil {
		return nil, fmt.Errorf("write netrc: %w", err)
	}
	err = runCommand(ctx, bio, "chmod", "600", netrcPath)
	if err != nil {
		// Not fatal. File will be removed later.
		log.Warnf(ctx, "Making temporary .netrc private: %v", err)
	}
	return biome.WithClose(bio, func() error {
		ctx := context.Background()
		err := runCommand(ctx, bio,
			"rm", bio.JoinPath(bio.Dirs().Home, netrcFilename))
		if err != nil {
			log.Warnf(ctx, "Could not clean up .netrc: %v", err)
		}
		return nil
	}), nil
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

// findPackage searches for the package configuration file in the current
// working directory or any parent directory. If the current working directory
// is a subdirectory of the package, subdir is the path of the working directory
// relative to pkg.Path.
func findPackage() (pkg *yb.Package, subdir string, err error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("find package configuration: %w", err)
	}
	for {
		pkg, err = yb.LoadPackage(filepath.Join(dir, yb.PackageConfigFilename))
		if err == nil {
			return pkg, subdir, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, "", fmt.Errorf("find package configuration: %w", err)
		}

		// Not found. Move up a directory.
		parent, name := filepath.Split(dir)
		if parent == dir {
			// Hit root.
			return nil, "", fmt.Errorf("find package configuration: %s not found in this or any parent directories", yb.PackageConfigFilename)
		}
		subdir = filepath.Join(name, subdir)
		dir = filepath.Clean(parent) // strip trailing separators
	}
}

func listTargetNames(targets map[string]*yb.Target) []string {
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// autocompleteTargetName provides tab completion suggestions for target names.
func autocompleteTargetName(toComplete string) ([]string, cobra.ShellCompDirective) {
	pkg, _, err := findPackage()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	names := make([]string, 0, len(pkg.Targets))
	for k := range pkg.Targets {
		if strings.HasPrefix(k, toComplete) {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	return names, cobra.ShellCompDirectiveNoFileComp
}
