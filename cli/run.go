package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/johnewart/subcommands"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/build"
	"github.com/yourbase/yb/internal/ybdata"
	"zombiezen.com/go/log"
)

type RunCmd struct {
	target      string
	noContainer bool
}

func (*RunCmd) Name() string     { return "run" }
func (*RunCmd) Synopsis() string { return "Run an arbitrary command" }
func (*RunCmd) Usage() string {
	return `run [-t TARGET] <COMMAND>
Run a command in the project container.
`
}

func (p *RunCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.target, "t", "default", "The target to run the command in")
	f.BoolVar(&p.noContainer, "no-container", false, "Avoid using Docker if possible")
}

func (b *RunCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) == 0 {
		fmt.Println(b.Usage())
		return subcommands.ExitFailure
	}
	dataDirs, err := ybdata.DirsFromEnv()
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	dockerClient, err := connectDockerClient(!b.noContainer)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	pkg, err := GetTargetPackage()
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	targets, err := pkg.Manifest.BuildOrder(b.target)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	dockerNetworkID, removeNetwork, err := newDockerNetwork(ctx, dockerClient)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	defer removeNetwork()

	// Build dependencies before running command.
	err = doTargetList(ctx, pkg, targets[:len(targets)-1], &doOptions{
		dockerClient:    dockerClient,
		dockerNetworkID: dockerNetworkID,
		dataDirs:        dataDirs,
	})
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}

	// Run command.
	execTarget := targets[len(targets)-1]
	bio, err := newBiome(ctx, dockerClient, pkg.Path)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	g := build.G{
		Biome:           bio,
		DockerClient:    dockerClient,
		DockerNetworkID: dockerNetworkID,
		Stdout:          os.Stdout,
		Stderr:          os.Stderr,
	}
	phaseDeps, err := targetToPhaseDeps(execTarget)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	execBiome, err := build.Setup(ctx, g, phaseDeps)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	// TODO(ch2744): Move this into build.Setup.
	err = pkg.SetupBuildDependencies(ctx, dataDirs, execTarget)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	// TODO(ch2725): Run the command from the subdirectory the process is in.
	err = execBiome.Run(ctx, &biome.Invocation{
		Argv:   f.Args(),
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}
