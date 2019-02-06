package main

import (
	"archive/tar"
	"context"
	"flag"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/johnewart/subcommands"

	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

func archiveSource(source, target string) error {
	return archiveDirectory(source, target, "workspace")
}

func copyArtifacts(source, target string) (int64, error) {
	targetBaseDir, err := filepath.Abs(target)

	if err != nil {
		return 0, err
	}
	log.Printf("Copying files to %s...\n", targetBaseDir)

	sourceBaseDir := filepath.Base(source)

	var totalBytes int64 = 0

	err = filepath.Walk(source,
		func(path string, info os.FileInfo, err error) error {
			dstPath := filepath.Join(targetBaseDir, info.Name())
			if err != nil {
				return err
			}

			if info.Name() == sourceBaseDir {
				log.Printf("Skipping base directory...")
				return nil
			}

			fmt.Printf("Copying %s to %s...\n", info.Name(), dstPath)
			sourceFileStat, err := os.Stat(path)
			if err != nil {
				return err
			}

			if sourceFileStat.Mode().IsDir() {
				os.Mkdir(dstPath, 0700)
				return nil
			}

			inFile, err := os.Open(path)
			if err != nil {
				return err
			}
			defer inFile.Close()

			outFile, err := os.Create(dstPath)
			if err != nil {
				return err
			}
			defer outFile.Close()
			nBytes, err := io.Copy(outFile, inFile)
			if err != nil {
				return err
			}

			totalBytes += nBytes
			return nil
		})

	if err != nil {
		log.Printf("Error walking directory: %v\n", err)
	}
	return totalBytes, err
}

func archiveArtifacts(source, target string) error {
	return archiveDirectory(source, target, "")
}

func archiveDirectory(source, target, prefix string) error {
	tarfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer tarfile.Close()

	tarball := tar.NewWriter(tarfile)
	defer tarball.Close()

	info, err := os.Stat(source)
	if err != nil {
		return nil
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
	}
	fmt.Printf("Base dir: %s\n", baseDir)

	return filepath.Walk(source,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}

			if prefix != "" {
				header.Name = filepath.Join(prefix, strings.TrimPrefix(path, source))
			} else {
				header.Name = filepath.Join(".", strings.TrimPrefix(path, source))
			}

			fmt.Printf("Adding %s as %s...\n", info.Name(), header.Name)

			if err := tarball.WriteHeader(header); err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(tarball, file)
			return err
		})

}

func (bc *BuildContext) doBuild() (bool, error) {
	fmt.Printf("Building in context id: %s\n", bc.Id)
	containerName := fmt.Sprintf("machinist-%s", bc.Id)
	instructions := bc.Instructions
	dockerClient := bc.DockerClient
	ctx := context.Background()

	imageName := instructions.Build.Image

	fmt.Printf("Pulling image '%s' if needed...\n", imageName)

	reader, err := dockerClient.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, reader)

	var mounts = make([]mount.Mount, 0)
	cachePath, err := filepath.Abs("../cache")
	fmt.Printf("CACHE DIRECTORY: %s\n", cachePath)
	if err != nil {
		return false, fmt.Errorf("Couldn't get absolute path to build directory!")
	}
	mounts = append(mounts, mount.Mount{
		Source: cachePath,
		Target: "/cache/",
		Type:   mount.TypeBind,
	})

	// Environment variables for the build tools' cache
	env := make([]string, 0)
	// Set Maven cache
	env = append(env, "MAVEN_OPTS=-Dmaven.repo.local=/cache/maven")
	// Bundler cache
	env = append(env, "BUNDLE_CACHE_PATH=/cache/bundler")
	//env = append(env, "M2_HOME=/cache/maven")
	env = append(env, "GOPATH=/cache/go")
	fmt.Printf("Creating build container '%s' using '%s'\n", containerName, instructions.Build.Image)
	var networkConfig = &network.NetworkingConfig{}
	var hostConfig = &container.HostConfig{
		Mounts: mounts,
	}

	var containerConfig = &container.Config{
		Image:        imageName,
		WorkingDir:   "/workspace/",
		Tty:          true,
		AttachStdout: true,
		Env:          env,
	}

	buildContainer, err := dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, containerName)
	if err != nil {
		panic(err)
	}

	dir, err := ioutil.TempDir("", "machinist-src")
	if err != nil {
		log.Fatal(err)
	}

	//defer os.RemoveAll(dir) // clean up
	tmpfile, err := os.OpenFile(fmt.Sprintf("%s/src.tar", dir), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0660)
	if err != nil {
		log.Fatal(err)
	}

	c := make(chan os.Signal, 1)

	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			log.Printf("Signal: %s", sig)
			// sig is a ^C, handle it
			log.Printf("CTRL-C hit, cleaning up!")
			// Terminate now, not in a bit
			stopTime, _ := time.ParseDuration("-1m")

			err := dockerClient.ContainerStop(ctx, buildContainer.ID, &stopTime)

			if err != nil {
				log.Printf("Unable to stop container...")
			}
			bc.Cleanup()
		}
	}()

	//defer os.Remove(tmpfile.Name())
	fmt.Printf("Archiving source to %s...\n", tmpfile.Name())
	archiveSource(".", tmpfile.Name())
	fmt.Printf("Done!\n")
	tarStream, err := os.Open(tmpfile.Name())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Adding source to container...\n")
	if err := dockerClient.CopyToContainer(ctx, buildContainer.ID, ".", tarStream, types.CopyToContainerOptions{}); err != nil {
		panic(err)
	}

	/*if err := dockerClient.ContainerStart(ctx, buildContainer.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}*/

	out, err := dockerClient.ContainerLogs(ctx, buildContainer.ID, types.ContainerLogsOptions{
		ShowStderr: true,
		ShowStdout: true,
		Timestamps: false,
		Follow:     true,
		Tail:       "40",
	})
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			io.Copy(os.Stdout, out)
		}
	}()

	log.Printf("Starting build container...\n")
	if err := dockerClient.ContainerStart(ctx, buildContainer.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	for _, cmd := range instructions.Build.Commands {
		fmt.Printf("Going to run '%s'...\n", cmd)
		exec, err := dockerClient.ContainerExecCreate(ctx, buildContainer.ID, types.ExecConfig{
			Cmd: strings.Split(cmd, " "),
		})

		if err != nil {
			panic(err)
		}

		fmt.Printf("Execution id: %s\n", exec.ID)
		err = dockerClient.ContainerExecStart(ctx, exec.ID, types.ExecStartCheck{})

		log.Printf("Waiting for exec to complete...")
		statusCh, errCh := dockerClient.ContainerWait(ctx, buildContainer.ID, container.WaitConditionNotRunning)
		select {
		case err := <-errCh:
			if err != nil {
				panic(err)
			}
		case <-statusCh:
		}
	}

	dir, err = ioutil.TempDir("", "machinist-artifacts")

	for _, element := range instructions.Build.Artifacts {
		downloadPath := fmt.Sprintf("/workspace/%s", element)
		//fileParts := strings.Split(element, "/")
		//		filename := fileParts[len(fileParts)-1]
		//		outfileName := fmt.Sprintf("%s/%s", dir, filename)

		if err != nil {
			log.Fatal(err)
		}

		dstPath := dir
		fmt.Printf("Extracting %s to %s...\n", element, dir)
		content, stat, err := dockerClient.CopyFromContainer(ctx, buildContainer.ID, downloadPath)
		if err != nil {
			return false, err
		}
		defer content.Close()

		srcInfo := archive.CopyInfo{
			Path:       downloadPath,
			Exists:     true,
			IsDir:      stat.Mode.IsDir(),
			RebaseName: "",
		}

		preArchive := content
		archive.CopyTo(preArchive, srcInfo, dstPath)
	}

	archiveArtifacts(dir, "artifacts.tar")
	err = os.Mkdir("build", 0700)
	if err != nil {
		log.Printf("build directory already exists!\n")
	}
	copyArtifacts(dir, "build")

	return true, nil
}

type buildCmd struct {
	capitalize bool
}

func (*buildCmd) Name() string     { return "build" }
func (*buildCmd) Synopsis() string { return "Build the workspace" }
func (*buildCmd) Usage() string {
	return `build`
}

func (p *buildCmd) SetFlags(f *flag.FlagSet) {
}

func (b *buildCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	workspace := LoadWorkspace()
	targetPackage := workspace.Target

	fmt.Printf("Building target package %s...\n", targetPackage)
	err := os.Chdir(targetPackage)
	if err != nil {
		return subcommands.ExitFailure
	}
	defer os.Chdir("..")

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.39"))
	if err != nil {
		panic(err)
	}

	containers, err := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	for _, container := range containers {
		fmt.Printf("%s %s\n", container.ID[:10], container.Image)
	}
	pwd, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		fmt.Errorf("Can't get PWD!\n")
	}

	ctx, err := NewContext(dockerClient, pwd)

	if err != nil {
		return subcommands.ExitFailure
	}

	// Do it
	defer ctx.Cleanup()
	ctx.doBuild()
	return subcommands.ExitSuccess

}
