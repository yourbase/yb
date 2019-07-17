package workspace

import (
	"fmt"
	docker "github.com/johnewart/go-dockerclient"
	"github.com/nu7hatch/gouuid"

	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	. "github.com/yourbase/yb/packages"
	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
)

type ServiceContext struct {
	DockerClient *docker.Client
	Id           string
	Containers   map[string]ContainerDefinition
	Package      Package
	NetworkId    string
}

type BuildContainerOpts struct {
	ContainerOpts ContainerDefinition
	Package       Package
	ExecUserId    string // who to run exec as (useful for local container builds which map the source)
	ExecGroupId   string
	MountPackage  bool
}

type BuildContainer struct {
	Id      string
	Name    string
	Options BuildContainerOpts
}

func (sc *ServiceContext) FindContainer(cd ContainerDefinition) (*BuildContainer, error) {
	return FindContainer(BuildContainerOpts{
		Package:       sc.Package,
		ContainerOpts: cd,
	})
}

// TODO: make sure the opts match the existing container
func FindContainer(opts BuildContainerOpts) (*BuildContainer, error) {

	client := NewDockerClient()
	cd := opts.ContainerOpts

	s := strings.Split(cd.Image, ":")
	imageName := s[0]
	containerName := fmt.Sprintf("%s-%s", opts.Package.Name, imageName)

	filters := make(map[string][]string)
	filters["name"] = append(filters["name"], containerName)

	result, err := client.ListContainers(docker.ListContainersOptions{
		Filters: filters,
		All:     true,
	})

	if err == nil && len(result) > 0 {
		for _, c := range result {
			fmt.Printf("ID: %s -- NAME: %s\n", c.ID, c.Names)
		}
		c := result[0]
		fmt.Printf("Found container %s with ID %s\n", containerName, c.ID)
		_, err := client.InspectContainer(c.ID)
		if err != nil {
			return nil, err
		} else {
			bc := BuildContainer{
				Id:      c.ID,
				Name:    containerName,
				Options: opts,
			}
			return &bc, nil
		}
	} else {
		return nil, err
	}

}

func StopContainerById(id string, timeout uint) error {
	client := NewDockerClient()
	fmt.Printf("Stopping container %s with a %d second timeout...\n", id, timeout)
	return client.StopContainer(id, timeout)
}

func RemoveContainerById(id string) error {
	client := NewDockerClient()
	return client.RemoveContainer(docker.RemoveContainerOptions{
		ID: id,
	})
}

func (sc *ServiceContext) NewContainer(c ContainerDefinition) (BuildContainer, error) {
	opts := BuildContainerOpts{
		ContainerOpts: c,
		Package:       sc.Package,
	}
	return NewContainer(opts)
}

func NewDockerClient() *docker.Client {
	// TODO: Do something smarter...
	endpoint := "unix:///var/run/docker.sock"
	client, err := docker.NewVersionedClient(endpoint, "1.39")
	if err != nil {
		panic(err)
	}
	return client

}

func PullImage(imageLabel string) error {
	client := NewDockerClient()
	filters := make(map[string][]string)

	parts := strings.Split(imageLabel, ":")
	imageName := parts[0]
	imageTag := ""

	if len(parts) == 2 {
		imageTag = parts[1]
	}

	fmt.Printf("Pulling %s if needed...\n", imageLabel)

	opts := docker.ListImagesOptions{
		Filters: filters,
	}

	imgs, err := client.ListImages(opts)
	if err != nil {
		fmt.Printf("Error getting image list: %v\n", err)
		return err
	}

	foundImage := false
	if len(imgs) > 0 {
		for _, img := range imgs {
			for _, tag := range img.RepoTags {
				if tag == imageLabel {
					fmt.Printf("Found image: %s with tags: %s\n", img.ID, strings.Join(img.RepoTags, ","))
					foundImage = true
				}
			}
		}
	}

	if !foundImage {
		fmt.Printf("Image %s not found, pulling\n", imageLabel)

		pullOpts := docker.PullImageOptions{
			Repository:   imageName,
			Tag:          imageTag,
			OutputStream: os.Stdout,
		}

		authConfig := docker.AuthConfiguration{}

		if err = client.PullImage(pullOpts, authConfig); err != nil {
			fmt.Printf("Unable to pull image: %v\n", err)
			return err
		}

	}

	return nil

}

func (b BuildContainer) Stop(timeout uint) error {
	client := NewDockerClient()
	fmt.Printf("Stopping container %s with a %d timeout...\n", b.Id, timeout)
	return client.StopContainer(b.Id, timeout)
}

func (b BuildContainer) Start() error {
	client := NewDockerClient()

	hostConfig := &docker.HostConfig{}

	return client.StartContainer(b.Id, hostConfig)
}

func (b BuildContainer) DownloadDirectoryToWriter(remotePath string, sink io.Writer) error {
	client := NewDockerClient()
	downloadOpts := docker.DownloadFromContainerOptions{
		OutputStream: sink,
		Path:         remotePath,
	}

	err := client.DownloadFromContainer(b.Id, downloadOpts)
	if err != nil {
		return fmt.Errorf("Unable to download %s: %v", remotePath, err)
	}

	return nil
}

func (b BuildContainer) DownloadDirectoryToFile(remotePath string, localFile string) error {
	outputFile, err := os.OpenFile(localFile, os.O_CREATE|os.O_RDWR, 0660)
	if err != nil {
		return fmt.Errorf("Can't create local file: %s: %v", localFile, err)
	}

	defer outputFile.Close()

	fmt.Printf("Downloading %s to %s...\n", remotePath, localFile)

	return b.DownloadDirectoryToWriter(remotePath, outputFile)
}

func (b BuildContainer) DownloadDirectory(remotePath string) (string, error) {

	dir, err := ioutil.TempDir("", "yb-container-download")

	if err != nil {
		return "", fmt.Errorf("Can't create temporary download location: %s: %v", dir, err)
	}

	fileParts := strings.Split(remotePath, "/")
	filename := fileParts[len(fileParts)-1]
	outfileName := fmt.Sprintf("%s.tar", filename)
	outfilePath := filepath.Join(dir, outfileName)

	err = b.DownloadDirectoryToFile(remotePath, outfilePath)

	if err != nil {
		return "", err
	}

	return outfilePath, nil
}

func (b BuildContainer) UploadStream(source io.Reader, remotePath string) error {
	client := NewDockerClient()

	uploadOpts := docker.UploadToContainerOptions{
		InputStream:          source,
		Path:                 remotePath,
		NoOverwriteDirNonDir: true,
	}

	err := client.UploadToContainer(b.Id, uploadOpts)

	return err
}

func (b BuildContainer) UploadArchive(localFile string, remotePath string) error {
	client := NewDockerClient()

	file, err := os.Open(localFile)
	if err != nil {
		return err
	}

	defer file.Close()

	uploadOpts := docker.UploadToContainerOptions{
		InputStream:          file,
		Path:                 remotePath,
		NoOverwriteDirNonDir: true,
	}

	err = client.UploadToContainer(b.Id, uploadOpts)

	return err
}

func (b BuildContainer) UploadFile(localFile string, fileName string, remotePath string) error {
	client := NewDockerClient()

	dir, err := ioutil.TempDir("", "yb")
	if err != nil {
		return err
	}

	defer os.RemoveAll(dir) // clean up
	tmpfile, err := os.OpenFile(fmt.Sprintf("%s/%s.tar", dir, fileName), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0660)
	if err != nil {
		return err
	}

	err = archiveFile(localFile, fileName, tmpfile.Name())

	if err != nil {
		return err
	}

	uploadOpts := docker.UploadToContainerOptions{
		InputStream:          tmpfile,
		Path:                 remotePath,
		NoOverwriteDirNonDir: true,
	}

	err = client.UploadToContainer(b.Id, uploadOpts)

	return err
}

func (b BuildContainer) CommitImage(repository string, tag string) (string, error) {
	client := NewDockerClient()

	commitOpts := docker.CommitContainerOptions{
		Container:  b.Id,
		Repository: repository,
		Tag:        tag,
	}

	img, err := client.CommitContainer(commitOpts)

	if err != nil {
		return "", err
	}

	fmt.Printf("Committed container %s as image %s:%s with id %s\n", b.Id, repository, tag, img.ID)

	return img.ID, nil
}

func (b BuildContainer) MakeDirectoryInContainer(path string) error {
	client := NewDockerClient()

	cmdArray := strings.Split(fmt.Sprintf("mkdir -p %s", path), " ")

	execOpts := docker.CreateExecOptions{
		Env:          b.Options.ContainerOpts.Environment,
		Cmd:          cmdArray,
		AttachStdout: true,
		AttachStderr: true,
		Container:    b.Id,
	}

	exec, err := client.CreateExec(execOpts)

	if err != nil {
		fmt.Printf("Can't create exec: %v\n", err)
		return err
	}

	startOpts := docker.StartExecOptions{
		OutputStream: os.Stdout,
		ErrorStream:  os.Stdout,
	}

	err = client.StartExec(exec.ID, startOpts)

	if err != nil {
		fmt.Printf("Unable to run exec %s: %v\n", exec.ID, err)
		return err
	}

	return nil

}

func (b BuildContainer) ExecToStdout(cmdString string, targetDir string) error {
	return b.ExecToWriter(cmdString, targetDir, os.Stdout)
}

func (b BuildContainer) ExecToWriter(cmdString string, targetDir string, outputSink io.Writer) error {
	client := NewDockerClient()

	shellCmd := []string{"bash", "-c", cmdString}

	execOpts := docker.CreateExecOptions{
		Env:          b.Options.ContainerOpts.Environment,
		Cmd:          shellCmd,
		AttachStdout: true,
		AttachStderr: true,
		Container:    b.Id,
		WorkingDir:   targetDir,
	}

	if b.Options.ExecUserId != "" || b.Options.ExecGroupId != "" {
		uidGid := fmt.Sprintf("%s:%s", b.Options.ExecUserId, b.Options.ExecGroupId)
		execOpts.User = uidGid
	}

	exec, err := client.CreateExec(execOpts)

	if err != nil {
		return fmt.Errorf("Can't create exec: %v", err)
	}

	startOpts := docker.StartExecOptions{
		OutputStream: outputSink,
		ErrorStream:  outputSink,
	}

	err = client.StartExec(exec.ID, startOpts)

	if err != nil {
		return fmt.Errorf("Unable to run exec %s: %v\n", exec.ID, err)
	}

	results, err := client.InspectExec(exec.ID)
	if err != nil {
		return fmt.Errorf("Unable to get exec results %s: %v\n", exec.ID, err)
	}

	if results.ExitCode != 0 {
		return fmt.Errorf("Command failed in container with status code %d", results.ExitCode)
	}

	return nil

}

func NewContainer(opts BuildContainerOpts) (BuildContainer, error) {
	containerDef := opts.ContainerOpts
	s := strings.Split(containerDef.Image, ":")
	imageName := s[0]
	containerName := fmt.Sprintf("%s-%s", opts.Package.Name, imageName)

	fmt.Printf("Creating container '%s'\n", containerName)

	client := NewDockerClient()

	PullImage(containerDef.Image)

	var mounts = make([]docker.HostMount, 0)

	buildRoot := opts.Package.BuildRoot()
	pkgWorkdir := filepath.Join(buildRoot, opts.Package.Name)

	for _, mountSpec := range containerDef.Mounts {
		s := strings.Split(mountSpec, ":")
		src := s[0]

		if !strings.HasPrefix(src, "/") {
			src = filepath.Join(pkgWorkdir, src)
			MkdirAsNeeded(src)
		}

		dst := s[1]

		fmt.Printf("Will mount %s as %s in container\n", src, dst)
		mounts = append(mounts, docker.HostMount{
			Source: src,
			Target: dst,
			Type:   "bind",
		})
	}

	if opts.MountPackage {
		sourceMapDir := "/workspace"
		if containerDef.WorkDir != "" {
			sourceMapDir = containerDef.WorkDir
		}

		fmt.Printf("Will mount package %s at %s in container\n", opts.Package.Path, sourceMapDir)
		mounts = append(mounts, docker.HostMount{
			Source: opts.Package.Path,
			Target: sourceMapDir,
			Type:   "bind",
		})
	}

	var ports = make([]string, 0)
	for _, portSpec := range containerDef.Ports {
		ports = append(ports, portSpec)
	}

	var bindings = make(map[docker.Port][]docker.PortBinding)
	for _, portSpec := range containerDef.Ports {
		parts := strings.Split(portSpec, ":")
		externalPort := parts[0]
		internalPort := parts[1]
		portKey := docker.Port(fmt.Sprintf("%s/tcp", internalPort))
		var pb = make([]docker.PortBinding, 0)
		pb = append(pb, docker.PortBinding{HostIP: "0.0.0.0", HostPort: externalPort})
		bindings[portKey] = pb
	}

	hostConfig := docker.HostConfig{
		Mounts:       mounts,
		PortBindings: bindings,
		Privileged:   containerDef.Privileged,
	}

	config := docker.Config{
		Env:          opts.ContainerOpts.Environment,
		AttachStdout: false,
		AttachStdin:  false,
		Image:        containerDef.Image,
		PortSpecs:    ports,
	}

	if len(opts.ContainerOpts.Command) > 0 {
		cmd := opts.ContainerOpts.Command
		fmt.Printf("Will run %s in the container\n", cmd)
		cmdParts := strings.Split(cmd, " ")
		config.Cmd = cmdParts
	}

	container, err := client.CreateContainer(
		docker.CreateContainerOptions{
			Name:       containerName,
			Config:     &config,
			HostConfig: &hostConfig,
		})

	if err != nil {
		fmt.Printf("Container failed to create: %v\n", err)
		return BuildContainer{}, err
	}

	fmt.Printf("Container ID: %s\n", container.ID)

	return BuildContainer{
		Name:    containerName,
		Id:      container.ID,
		Options: opts,
	}, nil
}

func NewServiceContext(pkg Package, containers map[string]ContainerDefinition) (*ServiceContext, error) {
	dockerClient := NewDockerClient()
	ctxId, _ := uuid.NewV4()
	/*log.Printf("Creating network %s...\n", ctxId.String())
	opts := types.NetworkCreate{}
	response, err := dockerClient.NetworkCreate(ctx, ctxId.String(), opts)
	if err != nil {
		return nil, err
	}*/

	sc := &ServiceContext{
		Package:      pkg,
		Id:           ctxId.String(),
		DockerClient: dockerClient,
		Containers:   containers,
		NetworkId:    "",
	}

	return sc, nil
}

func (sc *ServiceContext) SetupNetwork() (string, error) {
	/*ctx := context.Background()
	dockerClient := sc.DockerClient
	opts := types.NetworkCreate{}

	log.Printf("Creating network %s...\n", sc.Id)
	response, err := dockerClient.NetworkCreate(ctx, sc.Id, opts)

	if err != nil {
		return "", err
	}

	return response.ID, nil
	*/
	return "", nil
}

func (sc *ServiceContext) Cleanup() error {
	/*
		ctx := context.Background()
		dockerClient := sc.DockerClient

		log.Printf("Removing network %s\n", sc.Id)
		err := dockerClient.NetworkRemove(ctx, sc.Id)

		if err != nil {
			return err
		}
	*/
	return nil

}

func (sc *ServiceContext) StandUp() error {
	buildRoot := sc.Package.BuildRoot()
	pkgWorkdir := filepath.Join(buildRoot, sc.Package.Name)
	MkdirAsNeeded(pkgWorkdir)
	logDir := filepath.Join(pkgWorkdir, "logs")
	MkdirAsNeeded(logDir)

	for _, c := range sc.Containers {
		fmt.Printf("  %s...\n", c.Image)

		container, err := sc.FindContainer(c)

		if err != nil {
			fmt.Printf("Problem searching for container: %v\n", err)
			return err
		}

		if container != nil {
			fmt.Printf("Container already exists, starting...\n")
			container.Start()
		} else {
			container, err := sc.NewContainer(c)
			if err != nil {
				return err
			}
			fmt.Printf("Created container: %s\n", container.Id)
			container.Start()
		}

		/*log.Printf("Connecting to network %s...\n", sc.NetworkId)
		if err := dockerClient.NetworkConnect(ctx, sc.NetworkId, dependencyContainer.ID, &network.EndpointSettings{}); err != nil {
			panic(err)
		}

		log.Printf("Starting container...\n")
		if err := dockerClient.ContainerStart(ctx, dependencyContainer.ID, types.ContainerStartOptions{}); err != nil {
			panic(err)
		}

		props, err := dockerClient.ContainerInspect(ctx, dependencyContainer.ID)

		if err != nil {
			panic(err)
		}

		networkSettings := props.NetworkSettings
		ipv4 := networkSettings.DefaultNetworkSettings.IPAddress
		log.Printf("Container IP: %s\n", ipv4)

		//TODO: stream logs from each dependency to the build dir
		containerLogFile := filepath.Join(logDir, fmt.Sprintf("%s.log", imageName))
		Name:
		f, err := os.Create(containerLogFile)

		if err != nil {
			fmt.Printf("Unable to write to log file %s: %v\n", containerLogFile, err)
			return err
		}

		out, err := dockerClient.ContainerLogs(ctx, dependencyContainer.ID, types.ContainerLogsOptions{
			ShowStderr: true,
			ShowStdout: true,
			Timestamps: false,
			Follow:     true,
			Tail:       "40",
		})
		if err != nil {
			fmt.Printf("Can't get log handle for container %s: %v\n", dependencyContainer.ID, err)
			return err
		}
		go func() {
			for {
				io.Copy(f, out)
				time.Sleep(300 * time.Millisecond)
			}
		}()
		*/
	}

	return nil
}

func archiveFileInMemory(source string, target string) (*tar.Reader, error) {
	var buf bytes.Buffer

	tarball := tar.NewWriter(&buf)
	defer tarball.Close()

	info, err := os.Stat(source)
	if err != nil {
		return nil, err
	}

	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return nil, err
	}

	header.Name = target

	fmt.Printf("Adding %s as %s...\n", info.Name(), header.Name)

	if err := tarball.WriteHeader(header); err != nil {
		return nil, err
	}

	fh, err := os.Open(source)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	_, err = io.Copy(tarball, fh)

	tarball.Close()

	tr := tar.NewReader(&buf)
	return tr, nil

}

func archiveFile(source string, target string, tarfile string) error {
	tf, err := os.Create(tarfile)
	if err != nil {
		return err
	}
	defer tf.Close()

	tarball := tar.NewWriter(tf)
	defer tarball.Close()

	info, err := os.Stat(source)
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}

	header.Name = target

	fmt.Printf("Adding %s as %s...\n", info.Name(), header.Name)

	if err := tarball.WriteHeader(header); err != nil {
		return err
	}

	fh, err := os.Open(source)
	if err != nil {
		return err
	}
	defer fh.Close()
	_, err = io.Copy(tarball, fh)

	tarball.Close()

	return nil

}
