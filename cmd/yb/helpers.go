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
	"strconv"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/config"
	"github.com/yourbase/yb/internal/ybdata"
	"zombiezen.com/go/log"
)

type executionMode int

const (
	noContainer  executionMode = -1
	preferHost   executionMode = 0
	useContainer executionMode = 1
)

func (mode executionMode) String() string {
	switch mode {
	case noContainer:
		return "no-container"
	case preferHost:
		return "host"
	case useContainer:
		return "container"
	default:
		return fmt.Sprint(int(mode))
	}
}

func (mode *executionMode) Set(s string) error {
	switch strings.ToLower(s) {
	case "no-container":
		*mode = noContainer
	case "host":
		*mode = preferHost
	case "container":
		*mode = useContainer
	default:
		return fmt.Errorf("invalid execution mode %q", s)
	}
	return nil
}

func (mode executionMode) Type() string {
	return "host|container|no-container"
}

// executionModeVar registers the --mode flag.
func executionModeVar(flags *pflag.FlagSet, mode *executionMode) {
	flags.Var(mode, "mode", "how to execute commands in target")
	flags.AddFlag(&pflag.Flag{
		Name:        "no-container",
		Value:       noContainerFlag{mode},
		Usage:       "Avoid using Docker if possible",
		DefValue:    "false",
		NoOptDefVal: "true",
		Hidden:      true,
	})
}

func connectDockerClient(mode executionMode) (*docker.Client, error) {
	if mode <= noContainer {
		return nil, nil
	}
	dockerClient, err := docker.NewVersionedClientFromEnv("1.39")
	if err != nil {
		return nil, err
	}
	return dockerClient, nil
}

type newBiomeOptions struct {
	packageDir string
	downloader *ybdata.Downloader
	dataDirs   *ybdata.Dirs
	baseEnv    biome.Environment
	netrcFiles []string

	executionMode   executionMode
	dockerClient    *docker.Client
	dockerNetworkID string
}

func newBiome(ctx context.Context, target *yb.Target, opts newBiomeOptions) (biome.BiomeCloser, error) {
	if target.UseContainer && opts.dockerClient == nil {
		return nil, fmt.Errorf("set up environment for target %s: docker required but unavailable", target.Name)
	}
	log.Debugf(ctx, "Checking for netrc data in %s",
		append(append([]string(nil), config.DefaultNetrcFiles()...), opts.netrcFiles...))
	netrc, err := config.CatFiles(config.DefaultNetrcFiles(), opts.netrcFiles)
	if err != nil {
		return nil, fmt.Errorf("set up environment for target %s: %w", target.Name, err)
	}
	if opts.executionMode < useContainer {
		l := biome.Local{
			PackageDir: opts.packageDir,
		}
		var err error
		l.HomeDir, err = opts.dataDirs.BuildHome(opts.packageDir, target.Name, l.Describe())
		if err != nil {
			return nil, fmt.Errorf("set up environment for target %s: %w", target.Name, err)
		}
		log.Debugf(ctx, "Home located at %s", l.HomeDir)
		if err := ensureKeychain(ctx, l); err != nil {
			return nil, fmt.Errorf("set up environment for target %s: %w", target.Name, err)
		}
		bio, err := injectNetrc(ctx, l, netrc)
		if err != nil {
			return nil, fmt.Errorf("set up environment for target %s: %w", target.Name, err)
		}
		return biome.EnvBiome{
			Biome: bio,
			Env:   opts.baseEnv,
		}, nil
	}

	home, err := opts.dataDirs.BuildHome(opts.packageDir, target.Name, biome.DockerDescriptor())
	if err != nil {
		return nil, fmt.Errorf("set up environment for target %s: %w", target.Name, err)
	}
	log.Debugf(ctx, "Home located at %s", home)
	tiniFile, err := opts.downloader.Download(ctx, biome.TiniURL)
	if err != nil {
		return nil, fmt.Errorf("set up environment for target %s: %w", target.Name, err)
	}
	defer tiniFile.Close()
	c, err := biome.CreateContainer(ctx, opts.dockerClient, &biome.ContainerOptions{
		PackageDir: opts.packageDir,
		HomeDir:    home,
		TiniExe:    tiniFile,
		Definition: target.Container,
		NetworkID:  opts.dockerNetworkID,
		PullOutput: os.Stderr,
	})
	if err != nil {
		return nil, fmt.Errorf("set up environment for target %s: %w", target.Name, err)
	}
	bio, err := injectNetrc(ctx, c, netrc)
	if err != nil {
		return nil, fmt.Errorf("set up environment for target %s: %w", target.Name, err)
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

func newDockerNetwork(ctx context.Context, client *docker.Client, mode executionMode, targets []*yb.Target) (string, func(), error) {
	if client == nil || !shouldCreateDockerNetwork(mode, targets) {
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

func shouldCreateDockerNetwork(mode executionMode, targets []*yb.Target) bool {
	if mode >= useContainer {
		return true
	}
	if mode <= noContainer {
		return false
	}
	for _, target := range targets {
		if target.UseContainer {
			return true
		}
	}
	return false
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

type noContainerFlag struct {
	mode *executionMode
}

func (f noContainerFlag) String() string {
	return strconv.FormatBool(*f.mode == noContainer)
}

func (f noContainerFlag) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	if v {
		*f.mode = noContainer
	} else {
		*f.mode = preferHost
	}
	return nil
}

func (f noContainerFlag) Type() string {
	return "bool"
}
