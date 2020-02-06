package runtime

import (
	"fmt"
	"github.com/yourbase/yb/plumbing/log"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"

	"github.com/yourbase/narwhal"
)

type ContainerTarget struct {
	Target
	Container   *narwhal.Container
	Environment []string
	workDir string
}

func (t *ContainerTarget) OS() Os {
	return Linux
}

func (t *ContainerTarget) Architecture() Architecture {
	return Amd64
}

func (t *ContainerTarget) ToolsDir() string {
	t.Container.MakeDirectoryInContainer("/opt/yb/tools")
	return "/opt/yb/tools"
}

func (t *ContainerTarget) PathExists(path string) bool {
	// Assume we can use stat for now
	statCmd := fmt.Sprintf("stat %s", path)

	err := t.Container.ExecToWriter(statCmd, "/", ioutil.Discard)
	if err != nil {
		if execerr, ok := err.(*narwhal.ExecError); ok {
			if execerr.ExitCode != 0 {
				return false
			} else {
				// Error but retcode zero... ?
				return true
			}
		}
		return false
	}

	return true
}

func (t *ContainerTarget) String() string {
	return fmt.Sprintf("Container ID: %s workDir: %s", t.Container.Id, t.workDir)
}

func (t* ContainerTarget) CacheDir() string {
	t.Container.MakeDirectoryInContainer("/opt/yb/cache")
	return "/opt/yb/cache"
}

func (t *ContainerTarget) PrependToPath(dir string) {
	pathSet := false
	for i, e := range(t.Environment) {
		parts := strings.Split(e, "=")
		k := parts[0]
		v := parts[1]
		if k == "PATH" {
			newpath := fmt.Sprintf("PATH=%s:%s", dir, v)
			t.Environment[i] = 	newpath
			pathSet = true
		}
	}

	if !pathSet {
		path := fmt.Sprintf("%s:/usr/bin:/bin:/sbin:/usr/sbin", dir)
		t.SetEnv("PATH", path)
	}
}

func (t *ContainerTarget) UploadFile(src string, dest string) error {
	parts := strings.Split(dest, "/")
	filename := parts[len(parts)-1]
	destDir := strings.Join(parts[0:len(parts)-1], "/")
	if destDir == "" {
		destDir = "/"
	}
	log.Infof("Uploading %s to %s in %s", src, filename, destDir)
	err := t.Container.UploadFile(src, filename, destDir)
	if err != nil {
		return fmt.Errorf("Couldn't upload file to container: %v", err)
	}
	log.Infof("Done")
	return  nil

}

func (t *ContainerTarget) DownloadFile(url string) (string, error) {
	// TODO: upload if locally found

	localFile, err := DownloadFileWithCache(url)
	parts := strings.Split(url, "/")
	filename := parts[len(parts)-1]
	outputFilename := fmt.Sprintf("/tmp/%s", filename)

	if err == nil {
		// Downloaded locally, inject
		log.Infof("Injecting locally cached file %s as %s", localFile, outputFilename)
		err = t.Container.UploadFile(localFile, filename, "/tmp")
	}

	// If download or injection failed, fallback
	if err != nil {
		log.Infof("Failed to download and inject file: %v", err)
		log.Infof("Will download via curl in container")
		p := Process{
			Command:     fmt.Sprintf("curl %s -o %s", url, outputFilename),
			Directory:   "/tmp",
			Interactive: false,
		}

		if err := t.Run(p); err != nil {
			return "", err
		}
	}

	return outputFilename, nil
}

func (t *ContainerTarget) Unarchive(src string, dst string) error {
	var command string
	t.Container.MakeDirectoryInContainer(dst)

	if strings.HasSuffix(src, "tar.gz") {
		command = fmt.Sprintf("tar zxf %s -C %s", src, dst)
	}

	p := Process{
		Command:     command,
		Interactive: false,
		Directory:   "/tmp",
		Environment: nil,
	}

	return t.Run(p)
}

func (t *ContainerTarget) WorkDir() string {
	return t.workDir
}

func (t *ContainerTarget) SetEnv(key string, value string) error {
	log.Infof("SETTING ENV: %s = %s", key, value)
	envString := fmt.Sprintf("%s=%s", key, value)
	if t.Environment == nil {
		t.Environment = make([]string, 0)
	}
	t.Environment = append(t.Environment, envString)
	log.Infof("Container environment: %v", t.Environment)
	return nil
}

func (t *ContainerTarget) Run(p Process) error {
	fmt.Printf("Running container process: %s\n", p.Command)

	p.Environment = append(p.Environment, t.Environment...)
	p.Environment = append(p.Environment, t.Container.Definition.Environment...)
	fmt.Printf("Process env: %v\n", p.Environment)

	if p.Interactive {
		return t.Container.ExecInteractivelyWithEnv(p.Command, p.Directory, p.Environment)
	} else {
		err := t.Container.ExecToStdoutWithEnv(p.Command, p.Directory, p.Environment)
		if execerr, ok := err.(*narwhal.ExecError); ok {
			return &TargetRunError{
				ExitCode: execerr.ExitCode,
				Message:  execerr.Message,
			}
		}

		return err
	}
}


func (t *ContainerTarget) WriteFileContents(contents string, remotepath string) error {
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
		t.UploadFile(tmpfile.Name(), remotepath)
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

func getOutboundIP() (net.IP, error){
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
				log.Errorf("accept failed: %s", err)
				continue
			}

			go func(tconn net.Conn, unixSocket string) {
				defer conn.Close()
				uconn, err := net.Dial("unix", unixSocket)
				if err != nil {
					log.Warnf("unix dial failed: %s", err)
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

