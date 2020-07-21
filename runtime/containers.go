package runtime

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"

	"github.com/yourbase/narwhal"
	"github.com/yourbase/yb/plumbing/log"
)

const (
	containerYbDir      = "/opt/yb"
	containerToolsDir   = containerYbDir + "/tools"
	containerCacheDir   = containerYbDir + "/cache"
	containerOutputDir  = containerYbDir + "/output"
	containerInjectDir  = containerYbDir + "/tmp"
	containerWorkDir    = "/workspace"
	containerRunnerHome = "/home/runner"
	containerPermFixCmd = "chown -R runner:runner " + containerWorkDir + " " + containerYbDir + " " + containerRunnerHome
)

type ContainerTarget struct {
	Container   *narwhal.Container
	Environment []string
	workDir     string
	runtime     *Runtime
}

func (t *ContainerTarget) OS() Os {
	return Linux
}

func (t *ContainerTarget) OSVersion(ctx context.Context) string {
	var buf bytes.Buffer
	bufWriter := bufio.NewWriter(&buf)
	err := narwhal.ExecShell(ctx, narwhal.DockerClient(), t.Container.Id, "cat /etc/os-release", &narwhal.ExecShellOptions{
		Dir:            "/",
		CombinedOutput: bufWriter,
	})

	if err != nil {
		return "unknown"
	}

	err = bufWriter.Flush()
	if err != nil {
		return "unknown"
	}

	rd := bufio.NewReader(&buf)
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			return ""
		}
		if strings.HasPrefix(line, "VERSION_CODENAME") {
			line = strings.TrimSuffix(line, "\n")
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				return parts[1]
			}
		}
	}
	return ""

}

func (t *ContainerTarget) Architecture() Architecture {
	return Amd64
}

func (t *ContainerTarget) ToolsDir(ctx context.Context) string {
	err := t.MkdirAll(ctx, containerToolsDir)
	if err != nil {
		log.Errorf("Unable to create %s inside the container: %v", containerToolsDir, err)
		return ""
	}
	return containerToolsDir
}

func (t *ContainerTarget) ToolOutputSharedDir(ctx context.Context) string {
	err := t.MkdirAll(ctx, containerOutputDir)
	if err != nil {
		log.Errorf("Unable to create %s inside the container: %v", containerOutputDir, err)
		return ""
	}
	return containerOutputDir
}
func (t *ContainerTarget) PathExists(ctx context.Context, path string) bool {
	// Assume we can use stat for now
	statCmd := fmt.Sprintf("stat %s", path)

	err := narwhal.ExecShell(ctx, narwhal.DockerClient(), t.Container.Id, statCmd, nil)
	if err != nil {
		if code, _ := narwhal.IsExitError(err); code != 0 {
			return false
		}
		// Return false anyway, as it errored
		return false
	}

	return true
}

func (t *ContainerTarget) MkdirAll(ctx context.Context, path string) error {
	var uid, gid string
	if t.runtime.AsCurrentUser {
		uid = t.runtime.uid
		gid = t.runtime.gid
	}
	return narwhal.MkdirAllOwnedBy(ctx, narwhal.DockerClient(), t.Container.Id, containerToolsDir, uid, gid)
}

func (t *ContainerTarget) String() string {
	return fmt.Sprintf("Container ID: %s workDir: %s", t.Container.Id, t.workDir)
}

func (t *ContainerTarget) CacheDir(ctx context.Context) string {
	err := t.MkdirAll(ctx, containerCacheDir)
	if err != nil {
		return ""
	}
	return containerCacheDir
}

func (t *ContainerTarget) PrependToPath(ctx context.Context, dir string) {
	pathSet := false
	for i, e := range t.Environment {
		parts := strings.Split(e, "=")
		k := parts[0]
		v := parts[1]
		if k == "PATH" {
			newpath := fmt.Sprintf("PATH=%s:%s", dir, v)
			t.Environment[i] = newpath
			pathSet = true
		}
	}

	if !pathSet {
		path := fmt.Sprintf("%s:%s", dir, t.GetDefaultPath())
		t.SetEnv("PATH", path)
	}
}

func (t *ContainerTarget) GetDefaultPath() string {
	// TODO check other OS defaults, this works for Linux containers
	return "/usr/bin:/bin:/sbin:/usr/sbin"
}

func (t *ContainerTarget) UploadFile(ctx context.Context, src string, dest string) error {
	log.Infof("Uploading %s to %s", src, dest)
	err := narwhal.UploadFile(ctx, narwhal.DockerClient(), t.Container.Id, dest, src)
	if err != nil {
		return fmt.Errorf("uploading file to container: %v", err)
	}
	// Fix permissions after uploading
	if t.runtime.AsCurrentUser {
		err = t.Run(ctx, Process{Interactive: false, Output: nil, Command: containerPermFixCmd, Sudo: true})
		if err != nil {
			return err
		}
	}
	log.Infof("Done")
	return nil

}

func (t *ContainerTarget) DownloadFile(ctx context.Context, url string) (string, error) {
	// TODO: upload if locally found

	localFile, err := downloadFileWithCache(ctx, url)
	if err != nil {
		return "", err
	}
	if !strings.Contains(url, "/") {
		return "", fmt.Errorf("downloading URL %s: bad URL?", url)
	}

	err = t.MkdirAll(ctx, containerInjectDir)
	if err != nil {
		return "", err
	}

	parts := strings.Split(url, "/")
	filename := parts[len(parts)-1]
	outputFilename := containerInjectDir + "/" + filename

	// Downloaded locally, inject
	log.Infof("Injecting locally cached file %s as %s", localFile, outputFilename)
	err = t.UploadFile(ctx, outputFilename, localFile)

	// If download or injection failed, fallback
	if err != nil {
		log.Infof("Failed to download and inject file: %v", err)
		log.Infof("Will download via curl in container")
		p := Process{
			Command:     fmt.Sprintf("curl %s -o %s", url, outputFilename),
			Directory:   containerInjectDir,
			Interactive: false,
		}

		if err := t.Run(ctx, p); err != nil {
			return "", err
		}
	}

	return outputFilename, nil
}

func (t *ContainerTarget) Unarchive(ctx context.Context, src string, dst string) error {
	var command string
	err := narwhal.MkdirAll(ctx, narwhal.DockerClient(), t.Container.Id, dst)
	if err != nil {
		return fmt.Errorf("making dir for unarchiving %s: %v", src, err)
	}

	if strings.HasSuffix(src, "tar.gz") {
		command = fmt.Sprintf("tar zxf %s -C %s", src, dst)
	}

	if strings.HasSuffix(src, "tar.bz2") {
		command = fmt.Sprintf("tar jxf %s -C %s", src, dst)
	}

	p := Process{
		Command:     command,
		Interactive: false,
		Directory:   "/tmp",
		Environment: nil,
	}

	return t.Run(ctx, p)
}

func (t *ContainerTarget) WorkDir() string {
	if t.workDir == "" {
		t.workDir = containerWorkDir
	}
	return t.workDir
}

func (t *ContainerTarget) HomeDir() string {
	return containerRunnerHome
}

func (t *ContainerTarget) SetEnv(key string, value string) error {
	envString := fmt.Sprintf("%s=%s", key, value)
	if t.Environment == nil {
		t.Environment = make([]string, 0)
	}
	t.Environment = append(t.Environment, envString)
	return nil
}

func (t *ContainerTarget) Run(ctx context.Context, p Process) error {
	log.Infof("Running container process: %s\n", p.Command)

	p.Environment = append(p.Environment, t.Environment...)
	p.Environment = append(p.Environment, t.Container.Definition.Environment...)

	log.Debugf("Process env: %v", p.Environment)

	var output io.Writer

	output = os.Stdout
	if p.Output != nil {
		output = p.Output
	}

	t.Container.Definition.Environment = p.Environment
	opts := &narwhal.ExecShellOptions{
		Dir:            p.Directory,
		CombinedOutput: output,
		Env:            p.Environment,
	}

	if t.runtime.AsCurrentUser && !p.Sudo {
		cmd, err := newCommand(p.Command)
		if err != nil {
			return fmt.Errorf("parsing shell like syntax of: %s: %w", p.Command, err)
		}
		if cmd.valid && !cmd.cowPower {
			opts.UID = t.runtime.uid
			opts.GID = t.runtime.gid
			log.Debugf("Running as 'runner' %s:%s", opts.UID, opts.GID)
		}
	}

	if p.Sudo {
		log.Debugf("Running as root")
	}

	if p.Interactive {
		opts.Interactive = true
		return narwhal.ExecShell(ctx, narwhal.DockerClient(), t.Container.Id, p.Command, opts)
	} else {
		err := narwhal.ExecShell(ctx, narwhal.DockerClient(), t.Container.Id, p.Command, opts)
		if err != nil {
			if code, ok := narwhal.IsExitError(err); ok {
				return &TargetRunError{
					ExitCode: code,
					Message:  fmt.Sprintf("Error: %v", err),
				}
			}
		}

		return err
	}
}

func (t *ContainerTarget) WriteFileContents(ctx context.Context, contents string, remotepath string) error {
	if tmpfile, err := ioutil.TempFile("", "injection"); err != nil {
		log.Infof("Couldn't make temporary file: %v", err)
		return err
	} else {
		defer os.Remove(tmpfile.Name())

		if _, err := tmpfile.Write([]byte(contents)); err != nil {
			log.Warnf("Couldn't write data to file: %v", err)
			return err
		}

		tmpfile.Sync()

		log.Infof("Will inject %s as %s", tmpfile.Name(), remotepath)
		t.UploadFile(ctx, tmpfile.Name(), remotepath)
		tmpfile.Close()
	}

	return nil
}

func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func getDockerInterfaceIp() (string, error) {
	ip, err := getOutboundIP()
	if err != nil {
		return "", err
	}

	return ip.String(), nil
}

func getOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return net.IP{}, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP, nil
}

func ForwardUnixSocketToTcp(unixSocket string) (string, error) {
	port, err := GetFreePort()
	if err != nil {
		return "", err
	}

	dockerInterfaceIp, err := getDockerInterfaceIp()
	if err != nil {
		return "", err
	}

	listenAddr := fmt.Sprintf("%s:%d", dockerInterfaceIp, port)
	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("Forwarding %s on %s", unixSocket, listenAddr)

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Errorf("Accept failed: %s", err)
				continue
			}

			go func(tconn net.Conn, unixSocket string) {
				defer conn.Close()
				uconn, err := net.Dial("unix", unixSocket)
				if err != nil {
					log.Warnf("UNIX dial failed: %s", err)
					return
				}
				log.Infof("Opened %s", unixSocket)
				// copy tcp request -> unix socket
				go io.Copy(tconn, uconn)
				// copy unix socket -> tcp connection
				io.Copy(uconn, tconn)
				log.Infof("Done forwarding.")
			}(conn, unixSocket)
		}
	}()

	return listenAddr, nil
}

// runnerUser creates the 'runner' user, so build commands (not setup commands) will run as him/her
// and also sets up correct ownership, used by containerized executions/builds
func (r *Runtime) runnerUser(ctx context.Context, container *narwhal.Container) error {
	log.Debug("Sane Linux base setup:")
	for _, cmd := range []string{
		"groupadd -g " + r.gid + " -f runner",
		"bash -c 'id runner &>/dev/null || useradd -u " + r.uid + " -m -g " + r.gid + " runner'",
		"install -d " + containerRunnerHome + " -o runner -g runner",
		"bash -c 'stat " + containerWorkDir + " &>/dev/null || install -d " + containerWorkDir + " -o runner -g runner'",
		"install -d " + containerInjectDir + " -o runner -g runner",
		"bash -c 'stat " + containerYbDir + " &>/dev/null || install -d " + containerYbDir + " -o runner -g runner'",
		containerPermFixCmd,
	} {
		log.Debug(cmd)
		err := narwhal.ExecShell(ctx, narwhal.DockerClient(), container.Id, cmd, &narwhal.ExecShellOptions{CombinedOutput: nil, Dir: "/"})
		if err != nil {
			return fmt.Errorf("(re)creating runner with: '%s': %w", cmd, err)
		}
	}

	return nil
}
