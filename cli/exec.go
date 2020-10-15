package cli

import (
	"context"
	"flag"
	"os"

	"github.com/johnewart/subcommands"
	"github.com/yourbase/yb/internal/build"
	"github.com/yourbase/yb/internal/buildcontext"
	"github.com/yourbase/yb/internal/ybdata"
	"zombiezen.com/go/log"
)

const defaultExecEnvironment = "default"

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
	f.StringVar(&p.environment, "e", defaultExecEnvironment, "Environment to run as")
}

func (b *ExecCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	dataDirs, err := ybdata.DirsFromEnv()
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	const useDocker = true
	dockerClient, err := connectDockerClient(useDocker)
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
	pkg, err := GetTargetPackage()
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	bctx, err := newBuildContext(ctx, dockerClient, pkg.Path)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	g := build.G{
		Context:         bctx,
		DockerClient:    dockerClient,
		DockerNetworkID: dockerNetworkID,
		Stdout:          os.Stdout,
		Stderr:          os.Stderr,
	}
	defaultEnv, err := buildcontext.MapVars(pkg.Manifest.Exec.Environment[defaultExecEnvironment])
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	deps := &build.PhaseDeps{
		TargetName:          b.environment,
		Resources:           narwhalContainerMap(pkg.Manifest.Exec.Dependencies.Containers),
		EnvironmentTemplate: defaultEnv,
	}
	if b.environment != defaultExecEnvironment {
		overrideEnv, err := buildcontext.MapVars(pkg.Manifest.Exec.Environment[b.environment])
		if err != nil {
			log.Errorf(ctx, "%v", err)
			return subcommands.ExitFailure
		}
		for k, v := range overrideEnv {
			deps.EnvironmentTemplate[k] = v
		}
	}
	execContext, err := build.Setup(ctx, g, deps)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}
	g.Context = execContext
	defer func() {
		if err := execContext.Close(); err != nil {
			log.Errorf(ctx, "Clean up environment %s: %v", b.environment, err)
		}
	}()
	// TODO(ch2744): Move this into build.Setup.
	if err := pkg.SetupRuntimeDependencies(ctx, dataDirs); err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}

	err = build.Execute(ctx, g, &build.Phase{
		TargetName: b.environment,
		Commands:   pkg.Manifest.Exec.Commands,
	})
	return subcommands.ExitSuccess
}
