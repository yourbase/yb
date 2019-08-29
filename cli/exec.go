package cli

import (
	"context"
	"flag"
	"path/filepath"
	"strings"

	"github.com/johnewart/subcommands"
	log "github.com/sirupsen/logrus"

	. "github.com/yourbase/yb/packages"
	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
	. "github.com/yourbase/yb/workspace"
)

type ExecCmd struct {
	environment string
}

func (*ExecCmd) Name() string { return "exec" }
func (*ExecCmd) Synopsis() string {
	return "Execute a project in the workspace, defaults to target project"
}
func (*ExecCmd) Usage() string {
	return `exec [project]`
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
func (b *ExecCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	var targetPackage Package

	ActiveSection("Setup")
	if PathExists(MANIFEST_FILE) {
		currentPath, _ := filepath.Abs(".")
		_, packageName := filepath.Split(currentPath)
		pkg, err := LoadPackage(packageName, currentPath)
		if err != nil {
			log.Infof("Can't load package '%s': %v\n", packageName, err)
			return subcommands.ExitFailure
		}
		targetPackage = pkg
	} else {
		workspace, err := LoadWorkspace()
		if err != nil {
			log.Infof("Can't load workspace: %v\n", err)
			return subcommands.ExitFailure
		}
		pkg, err := workspace.TargetPackage()
		if err != nil {
			log.Infof("Can't determine target package: %v\n", err)
			return subcommands.ExitFailure
		}

		targetPackage = pkg
	}

	ActiveSection("Dependencies")
	if _, err := targetPackage.SetupRuntimeDependencies(); err != nil {
		log.Infof("Couldn't configure dependencies: %v\n", err)
		return subcommands.ExitFailure
	}

	instructions := targetPackage.Manifest
	containers := instructions.Exec.Dependencies.ContainerList()

	buildData := NewBuildData()

	contextId := targetPackage.Name
	if len(containers) > 0 {
		ActiveSection("Containers")
		log.Infof("Starting %d dependencies...", len(containers))
		sc, err := NewServiceContextWithId(contextId, targetPackage, containers)
		if err != nil {
			log.Errorf("Couldn't create service context for dependencies: %v", err)
			return subcommands.ExitFailure
		}

		if err = sc.StandUp(); err != nil {
			log.Infof("Couldn't start dependencies: %v\n", err)
			return subcommands.ExitFailure
		}

		buildData.Containers.ServiceContext = sc
	}

	ActiveSection("Environment")
	log.Infof("Setting environment variables...")
	for _, property := range instructions.Exec.Environment["default"] {
		s := strings.Split(property, "=")
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

	buildData.ExportEnvironmentPublicly()

	ActiveSection("Exec")

	log.Infof("Execing target package %s...\n", targetPackage.Name)

	execDir := targetPackage.Path

	for _, logFile := range instructions.Exec.LogFiles {
		log.Infof("Will tail %s...\n", logFile)
	}

	for _, cmdString := range instructions.Exec.Commands {
		if err := ExecToStdout(cmdString, execDir); err != nil {
			log.Infof("Failed to run %s: %v", cmdString, err)
			return subcommands.ExitFailure
		}
	}

	return subcommands.ExitSuccess
}
