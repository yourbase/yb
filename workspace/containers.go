package workspace

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	docker "github.com/johnewart/go-dockerclient"
	"github.com/nu7hatch/gouuid"

	. "github.com/yourbase/yb/packages"
	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"

	log "github.com/sirupsen/logrus"
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
	Namespace     string // A namespace for prefixing container names
}

type BuildContainer struct {
	Id      string
	Name    string
	Options BuildContainerOpts
}

func sanitizeContainerName(proposed string) string {
	// Remove unusable characters from the container name
	// Must match: [a-zA-Z0-9][a-zA-Z0-9_.-]
	re := regexp.MustCompile(`^([a-zA-Z0-9])([a-zA-Z0-9_.-]+)$`)

	if re.MatchString(proposed) {
		return proposed
	}

	badChars := regexp.MustCompile(`[^a-zA-Z0-9_.-]`)
	result := badChars.ReplaceAllString(proposed, "")

	firstCharRe := regexp.MustCompile(`[a-zA-Z0-9]`)
	if !firstCharRe.MatchString(string(result[0])) {
		result = result[1:]
	}

	return result
}

func (sc *ServiceContext) FindContainer(cd ContainerDefinition) (*BuildContainer, error) {
	return FindContainer(BuildContainerOpts{
		Package:       sc.Package,
		ContainerOpts: cd,
		Namespace:     sc.Id,
	})
}

// TODO: make sure the opts match the existing container
func FindContainer(opts BuildContainerOpts) (*BuildContainer, error) {

	client := NewDockerClient()
	cd := opts.ContainerOpts

	s := strings.Split(cd.Image, ":")
	imageName := s[0]

	containerName := fmt.Sprintf("%s-%s", opts.Package.Name, imageName)
	// Prefix container name with the namespace
	if opts.Namespace != "" {
		containerName = fmt.Sprintf("%s-%s", opts.Namespace, containerName)
	}

	filters := make(map[string][]string)
	filters["name"] = append(filters["name"], containerName)

	result, err := client.ListContainers(docker.ListContainersOptions{
		Filters: filters,
		All:     true,
	})

	if err == nil && len(result) > 0 {
		for _, c := range result {
			log.Debugf("ID: %s -- NAME: %s\n", c.ID, c.Names)
		}
		c := result[0]
		log.Debugf("Found container %s with ID %s\n", containerName, c.ID)
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
		Namespace:     sc.Id,
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

	log.Debugf("Pulling %s if needed...\n", imageLabel)

	opts := docker.ListImagesOptions{
		Filters: filters,
	}

	imgs, err := client.ListImages(opts)
	if err != nil {
		return fmt.Errorf("Error getting image list: %v", err)
	}

	foundImage := false
	if len(imgs) > 0 {
		for _, img := range imgs {
			for _, tag := range img.RepoTags {
				if tag == imageLabel {
					log.Debugf("Found image: %s with tags: %s\n", img.ID, strings.Join(img.RepoTags, ","))
					foundImage = true
				}
			}
		}
	}

	if !foundImage {
		log.Infof("Image %s not found, pulling\n", imageLabel)

		pullOpts := docker.PullImageOptions{
			Repository:   imageName,
			Tag:          imageTag,
			OutputStream: os.Stdout,
		}

		authConfig := docker.AuthConfiguration{}

		if err = client.PullImage(pullOpts, authConfig); err != nil {
			return fmt.Errorf("Unable to pull image: %v\n", err)
		}

	}

	return nil

}

func (b BuildContainer) ListNetworkIDs() ([]string, error) {
	client := NewDockerClient()
	c, err := client.InspectContainer(b.Id)

	networkIds := make([]string, 0)

	if err != nil {
		return networkIds, fmt.Errorf("Couldn't get networks for container %s: %v", b.Id, err)
	}

	for _, network := range c.NetworkSettings.Networks {
		networkIds = append(networkIds, network.NetworkID)
	}
	return networkIds, nil
}

func (b BuildContainer) DisconnectFromNetworks() error {

	dockerClient := NewDockerClient()
	if networkIds, err := b.ListNetworkIDs(); err != nil {
		return fmt.Errorf("Can't get listing of networks: %v", err)
	} else {
		for _, networkId := range networkIds {
			opts := docker.NetworkConnectionOptions{
				Container: b.Id,
				EndpointConfig: &docker.EndpointConfig{
					NetworkID: networkId,
				},
				Force: true,
			}

			if err := dockerClient.DisconnectNetwork(networkId, opts); err != nil {
				log.Warnf("Couldn't disconnect container %s from network %s: %v", b.Id, networkId, err)
			}
		}
	}

	return nil
}

func (b BuildContainer) EnsureRunning(uptime int) error {

	sleepTime := time.Duration(uptime) * time.Second
	time.Sleep(sleepTime)

	running, err := b.IsRunning()
	if err != nil {
		return fmt.Errorf("Couldn't wait for running state: %v", err)
	}

	if !running {
		return fmt.Errorf("Container stopped running before %d seconds", uptime)
	}

	return nil
}

func (b BuildContainer) WaitForTcpPort(port int, timeout int) error {
	address, err := b.IPv4Address()
	if err != nil {
		return fmt.Errorf("Couldn't wait for TCP port %d: %v", port, err)
	}

	hostPort := fmt.Sprintf("%s:%d", address, port)

	timeWaited := 0
	secondsToSleep := 1
	sleepTime := time.Duration(secondsToSleep) * time.Second

	for timeWaited < timeout {
		conn, err := net.Dial("tcp", hostPort)
		if err != nil {
			// Pass for now
			timeWaited = timeWaited + secondsToSleep
			time.Sleep(sleepTime)
		} else {
			conn.Close()
			return nil
		}
	}

	return fmt.Errorf("Couldn't connect to service before specified timeout (%d sec.)", timeout)
}

func (b BuildContainer) IPv4Address() (string, error) {
	client := NewDockerClient()
	c, err := client.InspectContainer(b.Id)

	if err != nil {
		return "", fmt.Errorf("Couldn't determine IP of container %s: %v", b.Id, err)
	}

	ipv4 := c.NetworkSettings.IPAddress
	return ipv4, nil
}

func (b BuildContainer) IsRunning() (bool, error) {
	client := NewDockerClient()
	c, err := client.InspectContainer(b.Id)
	if err != nil {
		return false, fmt.Errorf("Couldn't determine state of container %s: %v", b.Id, err)
	}

	return c.State.Running, nil
}

func (b BuildContainer) Stop(timeout uint) error {
	client := NewDockerClient()
	fmt.Printf("Stopping container %s with a %d timeout...\n", b.Id, timeout)
	return client.StopContainer(b.Id, timeout)
}

func (b BuildContainer) Start() error {
	client := NewDockerClient()

	if running, err := b.IsRunning(); err != nil {
		return fmt.Errorf("Couldn't determine if container %s is running: %v", b.Id, err)
	} else {
		if running {
			// Nothing to do
			return nil
		}
	}

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
	containerImageName := strings.Replace(imageName, "/", "_", -1)

	containerName := fmt.Sprintf("%s-%s", opts.Package.Name, containerImageName)

	if opts.Namespace != "" {
		containerName = fmt.Sprintf("%s-%s", opts.Namespace, containerName)
	}

	log.Infof("Creating container '%s'\n", containerName)

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

		log.Infof("Will mount %s as %s in container", src, dst)
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
		log.Debugf("Will run %s in the container", cmd)
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
		return BuildContainer{}, fmt.Errorf("Failed to create container: %v", err)
	}

	log.Debugf("Found container ID: %s\n", container.ID)

	return BuildContainer{
		Name:    containerName,
		Id:      container.ID,
		Options: opts,
	}, nil
}

func FindNetworkByName(name string) (*docker.Network, error) {
	dockerClient := NewDockerClient()
	log.Debugf("Finding network by name %s", name)
	filters := make(map[string]map[string]bool)
	filter := make(map[string]bool)
	filter[name] = true
	filters["name"] = filter
	networks, err := dockerClient.FilteredListNetworks(filters)

	if err != nil {
		return nil, fmt.Errorf("Can't filter networks by name %s: %v", name, err)
	}

	if len(networks) == 0 {
		return nil, nil
	}

	network := networks[0]
	return &network, nil
}

func NewServiceContextWithId(ctxId string, pkg Package, containers map[string]ContainerDefinition) (*ServiceContext, error) {
	dockerClient := NewDockerClient()
	fmt.Printf("Creating service context '%s'...\n", ctxId)

	// Find network by context Id

	network, err := FindNetworkByName(ctxId)

	if err != nil {
		return nil, fmt.Errorf("Couldn't find existing network: %v", err)
	}

	if err == nil && network == nil {
		opts := docker.CreateNetworkOptions{
			Name:   ctxId,
			Driver: "bridge",
		}

		network, err = dockerClient.CreateNetwork(opts)

		if err != nil {
			return nil, fmt.Errorf("Unable to create Docker network: %v", err)
		}
	}

	sc := &ServiceContext{
		Package:      pkg,
		Id:           ctxId,
		DockerClient: dockerClient,
		Containers:   containers,
		NetworkId:    network.ID,
	}

	return sc, nil
}

func NewServiceContext(pkg Package, containers map[string]ContainerDefinition) (*ServiceContext, error) {
	ctxId, _ := uuid.NewV4()
	return NewServiceContextWithId(ctxId.String(), pkg, containers)
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
func (sc *ServiceContext) TearDown() error {
	fmt.Printf("Terminating services...\n")

	for _, c := range sc.Containers {
		fmt.Printf("  %s...", c.Image)

		container, err := sc.FindContainer(c)

		if err != nil {
			fmt.Printf("Problem searching for container: %v\n", err)
		}

		if container != nil {
			fmt.Printf(" %s\n", container.Id)
			container.Stop(0)
		}
	}

	client := sc.DockerClient
	fmt.Printf("Removing network...\n")
	err := client.RemoveNetwork(sc.NetworkId)
	if err != nil {
		fmt.Printf("Unable to remove network %s: %v", sc.NetworkId, err)
	}

	return nil
}

func (sc *ServiceContext) StandUp() error {
	dockerClient := sc.DockerClient
	buildRoot := sc.Package.BuildRoot()
	pkgWorkdir := filepath.Join(buildRoot, sc.Package.Name)
	MkdirAsNeeded(pkgWorkdir)
	logDir := filepath.Join(pkgWorkdir, "logs")
	MkdirAsNeeded(logDir)

	log.Infof("Starting up dependent containers and network...")

	for label, c := range sc.Containers {
		log.Infof("  ===  %s ===", c.Image)

		container, err := sc.FindContainer(c)

		if err != nil {
			return fmt.Errorf("Problem searching for container %s: %v", c.Image, err)
		}

		if container != nil {
			log.Infof("Container for %s already exists, not re-creating...", c.Image)
		} else {
			c, err := sc.NewContainer(c)
			if err != nil {
				return err
			}
			container = &c
			log.Infof("Created container: %s\n", container.Id)

			// Disconnect from existing networks if needed
			container.DisconnectFromNetworks()

			// Attach to network
			log.Infof("Attaching container to network ... ")
			opts := docker.NetworkConnectionOptions{
				Container: container.Id,
				EndpointConfig: &docker.EndpointConfig{
					NetworkID: sc.NetworkId,
				},
			}

			if err = dockerClient.ConnectNetwork(sc.NetworkId, opts); err != nil {
				return fmt.Errorf("Couldn't connect container %s to network %s: %v", container.Id, sc.NetworkId, err)
			}

		}

		running, err := container.IsRunning()
		if err != nil {
			return fmt.Errorf("Couldn't determine if container is running: %v", err)
		}

		if !running {
			log.Infof("Starting container for %s...", c.Image)
			if err = container.Start(); err != nil {
				return fmt.Errorf("Couldn't start container %s: %v", container.Id, err)
			}
		}

		ipv4, err := container.IPv4Address()
		if err != nil {
			return fmt.Errorf("Couldn't determine IP of container dependency %s (%s): %v", label, container.Id, err)
		}

		if ipv4 == "" {
			return fmt.Errorf("Container didn't get an IP address -- check the logs for container %s", container.Id[0:12])
		}
		log.Infof("Container IP: %s", ipv4)

		if c.PortWaitCheck.Port != 0 {
			fmt.Printf(" Waiting for %s:%d for %d sec... ", ipv4, c.PortWaitCheck.Port, c.PortWaitCheck.Timeout)
			if err := container.WaitForTcpPort(c.PortWaitCheck.Port, c.PortWaitCheck.Timeout); err != nil {
				fmt.Printf(" timed out!")
			} else {
				fmt.Printf(" OK!")
			}
		}

		fmt.Println()

		/*//TODO: stream logs from each dependency to the build dir
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
