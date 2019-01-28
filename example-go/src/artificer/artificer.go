package main

import (
	"archive/tar"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"github.com/nu7hatch/gouuid"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type BuildInstructions struct {
	Build     BuildPhase     `yaml:"build"`
	Container ContainerPhase `yaml:"container"`
}

type BuildPhase struct {
	Image     string   `yaml:"image"`
	Command   string   `yaml:"command"`
	Sandbox   bool     `yaml:"sandbox"`
	Artifacts []string `yaml:"artifacts"`
}

type ContainerPhase struct {
	BaseImage string   `yaml:"base_image"`
	Command   string   `yaml:"command"`
	Artifacts []string `yaml:"artifacts"`
}

func archiveSource(source, target string) error {
	return archiveDirectory(source, target, "workspace")
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

func main() {
	dockerClient, _ := docker.NewClient("unix:///var/run/docker.sock")

	imgs, _ := dockerClient.ListImages(docker.ListImagesOptions{All: true})
	for _, img := range imgs {
		fmt.Println("ID: ", img.ID)
		fmt.Println("RepoTags: ", img.RepoTags)
		fmt.Println("Created: ", img.Created)
		fmt.Println("Size: ", img.Size)
		fmt.Println("VirtualSize: ", img.VirtualSize)
		fmt.Println("ParentId: ", img.ParentID)
	}

	instructions := BuildInstructions{}

	buildyaml, _ := ioutil.ReadFile("build.yml")

	err := yaml.Unmarshal([]byte(buildyaml), &instructions)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	fmt.Printf("--- i:\n%v\n\n", instructions)

	u, err := uuid.NewV4()
	containerName := fmt.Sprintf("machinist-%s", u)
	imageName := instructions.Build.Image

	auth := docker.AuthConfiguration{}

	pullOpts := docker.PullImageOptions{
		Repository:   imageName,
		Registry:     "",
		Tag:          "",
		OutputStream: os.Stdout,
	}

	fmt.Printf("Pulling image '%s' if needed...\n", imageName)

	dockerClient.PullImage(pullOpts, auth)

	fmt.Printf("Creating build container '%s' using '%s'\n", containerName, instructions.Build.Image)

	createOpts := docker.CreateContainerOptions{
		Name: containerName,
		Config: &docker.Config{
			Image:        instructions.Build.Image,
			Cmd:          strings.Split(instructions.Build.Command, " "),
			AttachStdout: true,
			WorkingDir:   "/workspace",
		},
		HostConfig: &docker.HostConfig{},
	}

	container, err := dockerClient.CreateContainer(createOpts)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	fmt.Printf("Build container: %s\n", container.ID)
	fmt.Printf("Going to run '%s'...\n", instructions.Build.Command)

	dir, err := ioutil.TempDir("", "machinist-src")
	if err != nil {
		log.Fatal(err)
	}

	defer os.RemoveAll(dir) // clean up
	tmpfile, err := os.OpenFile(fmt.Sprintf("%s/src.tar", dir), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0660)
	if err != nil {
		log.Fatal(err)
	}

	//defer os.Remove(tmpfile.Name())
	fmt.Printf("Archiving source to %s...\n", tmpfile.Name())
	archiveSource("src", tmpfile.Name())

	tarStream, err := os.Open(tmpfile.Name())
	if err != nil {
		log.Fatal(err)
	}

	dockerClient.UploadToContainer(container.ID, docker.UploadToContainerOptions{
		InputStream: tarStream,
		Path:        "/",
	})

	err = dockerClient.StartContainer(container.ID, nil)

	if err != nil {
		log.Fatal(err)
	}

	logsOpts := docker.LogsOptions{
		Container:    container.ID,
		OutputStream: os.Stdout,
		Stdout:       true,
		Stderr:       true,
		Follow:       true,
	}

	err = dockerClient.Logs(logsOpts)

	if err != nil {
		log.Fatal(err)
	}

	dir, err = ioutil.TempDir("", "machinist-artifacts")

	for _, element := range instructions.Build.Artifacts {
		downloadPath := fmt.Sprintf("/workspace/%s", element)
		fileParts := strings.Split(element, "/")
		filename := fileParts[len(fileParts)-1]
		outfileName := fmt.Sprintf("%s/%s", dir, filename)
		outputFile, err := os.OpenFile(outfileName, os.O_CREATE|os.O_RDWR, 0660)

		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Extracting %s to %s...\n", element, outfileName)

		downloadOpts := docker.DownloadFromContainerOptions{
			OutputStream: outputFile,
			Path:         downloadPath,
		}

		err = dockerClient.DownloadFromContainer(container.ID, downloadOpts)

		if err != nil {
			log.Fatal(err)
		}
	}

	archiveArtifacts(dir, "artifacts.tar")

}
