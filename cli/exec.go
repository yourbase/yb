package cli

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/johnewart/subcommands"
	"github.com/yourbase/narwhal"
	"github.com/yourbase/yb/internal/ybdata"
	"github.com/yourbase/yb/plumbing"
	"zombiezen.com/go/log"
)

type ExecCmd struct {
	environment string
}

func (*ExecCmd) Name() string { return "exec" }
func (*ExecCmd) Synopsis() string {
	return "Execute a project in the workspace, defaults to target project"
}
func (*ExecCmd) Usage() string {
	return `Usage: exec [PROJECT]
Execute a project in the workspace, as specified by .yourbase.yml's exec block.
`
}

func (p *ExecCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.environment, "e", "", "Environment to run as")
}

/*
Executing the target involves:
1. Map source into the target container
2. Run any dependent components
3. Start target
*/
func (b *ExecCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	dataDirs, err := ybdata.FromEnv()
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}

	targetPackage, err := GetTargetPackage()
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}

	if err := targetPackage.SetupRuntimeDependencies(ctx, dataDirs); err != nil {
		log.Infof(ctx, "Couldn't configure dependencies: %v\n", err)
		return subcommands.ExitFailure
	}

	instructions := targetPackage.Manifest
	containers := instructions.Exec.Dependencies.ContainerList()

	buildData := NewBuildData()

	if len(containers) > 0 {
		dockerClient, err := docker.NewVersionedClient("unix:///var/run/docker.sock", "1.39")
		if err != nil {
			log.Errorf(ctx, "%v", err)
			return subcommands.ExitFailure
		}

		localContainerWorkDir := filepath.Join(targetPackage.BuildRoot(), "containers")
		if err := os.MkdirAll(localContainerWorkDir, 0777); err != nil {
			log.Errorf(ctx, "Couldn't create directory: %v", err)
			return subcommands.ExitFailure
		}

		log.Infof(ctx, "Will use %s as the dependency work dir", localContainerWorkDir)
		log.Infof(ctx, "Starting %d dependencies...", len(containers))
		sc, err := narwhal.NewServiceContextWithId(ctx, dockerClient, "exec", targetPackage.BuildRoot())
		if err != nil {
			log.Errorf(ctx, "Couldn't create service context for dependencies: %v", err)
			return subcommands.ExitFailure
		}

		for _, c := range containers {
			// TODO: avoid setting these here
			c.LocalWorkDir = localContainerWorkDir
			if _, err = sc.StartContainer(ctx, os.Stderr, c); err != nil {
				log.Infof(ctx, "Couldn't start dependencies: %v\n", err)
				return subcommands.ExitFailure
			}
		}

		buildData.Containers.ServiceContext = sc
	}

	log.Infof(ctx, "Setting environment variables...")
	for _, property := range instructions.Exec.Environment["default"] {
		s := strings.SplitN(property, "=", 2)
		if len(s) == 2 {
			buildData.SetEnv(s[0], s[1])
		}
	}

	if b.environment != "default" {
		for _, property := range instructions.Exec.Environment[b.environment] {
			s := strings.Split(property, "=")
			if len(s) == 2 {
				buildData.SetEnv(s[0], s[1])
			}
		}
	}

	buildData.ExportEnvironmentPublicly(ctx)

	log.Infof(ctx, "Execing target package %s...\n", targetPackage.Name)

	execDir := targetPackage.Path

	for _, logFile := range instructions.Exec.LogFiles {
		log.Infof(ctx, "Will tail %s...\n", logFile)
	}

	for _, cmdString := range instructions.Exec.Commands {
		if err := plumbing.ExecToStdout(cmdString, execDir); err != nil {
			log.Errorf(ctx, "Failed to run %s: %v", cmdString, err)
			return subcommands.ExitFailure
		}
	}

	return subcommands.ExitSuccess
}
