package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/johnewart/subcommands"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type buildCmd struct {
	ExecPrefix  string
	NoContainer bool
}

func (*buildCmd) Name() string     { return "build" }
func (*buildCmd) Synopsis() string { return "Build the workspace" }
func (*buildCmd) Usage() string {
	return `build`
}

func (p *buildCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&p.NoContainer, "no-container", false, "Bypass container even if specified")
	f.StringVar(&p.ExecPrefix, "exec-prefix", "", "Add a prefix to all executed commands (useful for timing or wrapping things)")
}

func (b *buildCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	realStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputs := io.MultiWriter(realStdout)
	var buf bytes.Buffer
	uploadBuildLogs := false

	if v, err := GetConfigValue("user", "upload_build_logs"); err == nil {
		fmt.Printf("Upload build logs set to: %s\n", v)
		if v == "true" {
			uploadBuildLogs = true
			outputs = io.MultiWriter(realStdout, &buf)
		}
	}

	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		for {
			io.Copy(outputs, r)
		}
	}()

	defer w.Close()
	defer r.Close()

	workspace := LoadWorkspace()
	buildTarget := "default"
	var targetPackage string

	if PathExists(MANIFEST_FILE) {
		currentPath, _ := filepath.Abs(".")
		_, file := filepath.Split(currentPath)
		targetPackage = file
	} else {
		targetPackage = workspace.Target
	}

	if len(f.Args()) > 0 {
		buildTarget = f.Args()[0]
	}

	targetDir := workspace.PackagePath(targetPackage)

	fmt.Printf("Building target package %s in %s...\n", targetPackage, targetDir)
	instructions, err := workspace.LoadPackageManifest(targetPackage)

	if err != nil {
		fmt.Printf("Unable to load package manifest for %s: %v\n", targetPackage, err)
		return subcommands.ExitFailure
	}

	fmt.Printf("Working in %s...\n", targetDir)

	var target BuildPhase
	sandboxed := instructions.Sandbox || target.Sandbox

	if len(instructions.BuildTargets) == 0 {
		target = instructions.Build
		if len(target.Commands) == 0 {
			fmt.Printf("Default build command has no steps and no targets described\n")
		}
	} else {
		ok := false
		if target, ok = instructions.BuildTargets[buildTarget]; !ok {
			targets := make([]string, 0, len(instructions.BuildTargets))
			for t := range instructions.BuildTargets {
				targets = append(targets, t)
			}

			fmt.Printf("Build target %s specified but it doesn't exist!\n")
			fmt.Printf("Valid build targets: %s\n", strings.Join(targets, ", "))
		}
	}

	// Set any environment variables as the last thing (override things we do in case people really want to do this)
	for _, envString := range target.Environment {
		parts := strings.Split(envString, "=")
		key := parts[0]
		value := parts[1]
		value = strings.Replace(value, "{PKGDIR}", targetDir, -1)
		fmt.Printf("Setting %s = %s\n", key, value)
		os.Setenv(key, value)
	}

	// If the build specifies a container and the --no-container flag isn't true
	if target.Container.Image != "" && !b.NoContainer {
		// Perform build inside a container
		image := target.Container.Image
		fmt.Printf("Invoking build in a container: %s\n", image)

		buildOpts := BuildContainerOpts{
			ContainerOpts: target.Container,
			PackageName:   targetPackage,
			Workspace:     workspace,
		}

		var buildContainer BuildContainer

		existing, err := FindContainer(buildOpts)

		if err != nil {
			fmt.Printf("Failed trying to find container: %v\n", err)
		}

		if existing != nil {
			fmt.Printf("Found existing container %s, removing...\n", existing.Id)
			if err = RemoveContainerById(existing.Id); err != nil {
				fmt.Printf("Unable to remove existing container: %v\n", err)
			}
		}

		buildContainer, err = NewContainer(buildOpts)
		if err != nil {
			fmt.Printf("Error creating build container: %v\n", err)
			return subcommands.ExitFailure
		}

		if err := buildContainer.Start(); err != nil {
			fmt.Printf("Unable to start container %s: %v", buildContainer.Id, err)
			return subcommands.ExitFailure
		}

		fmt.Printf("Building in container: %s\n", buildContainer.Id)

		for _, cmdString := range target.Commands {
			if len(b.ExecPrefix) > 0 {
				cmdString = fmt.Sprintf("%s %s", b.ExecPrefix, cmdString)
			}

			fmt.Printf("Running %s in the container\n", cmdString)

			if err := buildContainer.ExecToStdout(cmdString); err != nil {
				fmt.Printf("Failed to run %s: %v", cmdString, err)
				return subcommands.ExitFailure
			}
		}

	} else {

		// Ensure build deps are :+1:
		workspace.SetupBuildDependencies(*instructions)
		for _, cmdString := range target.Commands {

			if len(b.ExecPrefix) > 0 {
				cmdString = fmt.Sprintf("%s %s", b.ExecPrefix, cmdString)
			}

			if strings.HasPrefix(cmdString, "cd ") {
				parts := strings.SplitN(cmdString, " ", 2)
				dir := filepath.Join(targetDir, parts[1])
				//fmt.Printf("Chdir to %s\n", dir)
				//os.Chdir(dir)
				targetDir = dir
			} else {
				if target.Root != "" {
					fmt.Printf("Build root is %s\n", target.Root)
					targetDir = filepath.Join(targetDir, target.Root)
				}

				if sandboxed {
					fmt.Println("Running build in a sandbox!")
					if err := ExecInSandbox(cmdString, targetDir); err != nil {
						fmt.Printf("Failed to run %s: %v", cmdString, err)
						return subcommands.ExitFailure
					}
				} else {
					if err := ExecToStdout(cmdString, targetDir); err != nil {
						fmt.Printf("Failed to run %s: %v", cmdString, err)
						return subcommands.ExitFailure
					}
				}
			}
		}

	}

	os.Stdout = realStdout

	if uploadBuildLogs {
		fmt.Println("UPLOADING...")
		buildLog := BuildLog{
			Contents: buf.String(),
		}
		jsonData, _ := json.Marshal(buildLog)
		resp, err := postJsonToApi("/buildlogs", jsonData)
		if err != nil {
			fmt.Printf("Couldn't upload logs: %v\n", err)
		}

		if resp.StatusCode != 200 {
			fmt.Printf("Status code uploading log: %d\n", resp.StatusCode)
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				fmt.Printf("Couldn't read response body: %s\n", err)
			}
			fmt.Println(string(body))
		}
	}

	return subcommands.ExitSuccess

}

type BuildLog struct {
	Contents string `json:"contents"`
}
