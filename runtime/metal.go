package runtime

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/google/shlex"
	"github.com/johnewart/archiver"
	"github.com/matishsiao/goInfo"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
)

type MetalTarget struct {
	workDir string
}

func (t *MetalTarget) OS() Os {
	switch goruntime.GOOS {
	case "linux":
		return Linux
	case "darwin":
		return Darwin
	case "windows":
		return Windows
	}

	log.Fatal("Running on an unknown OS - things will likely fail miserably...")
	return Unknown
}

func (t *MetalTarget) OSVersion(ctx context.Context) string {
	info := goInfo.GetInfo()
	if info.Core != "" {
		return info.Core
	}
	return "unknown"
}

func (t *MetalTarget) Architecture() Architecture {
	return Amd64
}

func (t *MetalTarget) WriteContentsToFile(contents string, filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}

	defer f.Close()

	if _, err := f.Write([]byte(contents)); err != nil {
		return err
	}

	return nil
}

func (t *MetalTarget) ToolsDir(ctx context.Context) string {
	toolsDir, exists := os.LookupEnv("YB_TOOLS_DIR")
	if !exists {
		u, err := user.Current()
		if err != nil {
			toolsDir = "/tmp/yourbase/tools"
		} else {
			toolsDir = fmt.Sprintf("%s/.yourbase/tools", u.HomeDir)
		}
	}

	plumbing.MkdirAsNeeded(toolsDir)

	return toolsDir
}

func (t *MetalTarget) PathExists(ctx context.Context, path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (t *MetalTarget) String() string {
	return fmt.Sprintf("Metal target: %s", t.workDir)
}

func (t *MetalTarget) PrependToPath(ctx context.Context, dir string) {
	currentPath := t.GetDefaultPath()
	// Only prepend if it's not already the head; presume that
	// whomever asked for this wants to be at the front so it's okay if it's
	// duplicated later
	if !strings.HasPrefix(currentPath, dir) {
		newPath := fmt.Sprintf("%s:%s", dir, currentPath)
		t.SetEnv("PATH", newPath)
	}
}

func (t *MetalTarget) GetDefaultPath() string {
	// TODO check other OS defaults, this works for Linux containers, maybe for a Mac host we should use "path"
	return os.Getenv("PATH")
}

// CacheDir returns a local filesystem cache path to hold tools distributed archives
func (t *MetalTarget) CacheDir(ctx context.Context) string {
	dir := localCacheDir()
	t.MkdirAsNeeded(ctx, dir)

	return dir
}

func (t *MetalTarget) UploadFile(ctx context.Context, src string, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}

	destination, err := os.Open(dst)
	if err != nil {
		return err
	}

	buf := make([]byte, 128*1024)
	for {
		n, err := source.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}

		if _, err := destination.Write(buf[:n]); err != nil {
			return err
		}
	}

	source.Close()
	destination.Close()

	return nil
}

func (t *MetalTarget) DownloadFile(ctx context.Context, url string) (string, error) {

	localFile, err := downloadFileWithCache(ctx, url)

	if err != nil {
		log.Errorf("Unable to download: %v", err)
	}

	return localFile, err
}

func (t *MetalTarget) Unarchive(ctx context.Context, src string, dst string) error {
	err := archiver.Unarchive(src, dst)

	if err != nil {
		log.Errorf("Unable to decompress: %v", err)
	}

	return err
}

func (t *MetalTarget) WorkDir() string {
	return t.workDir
}

func (t *MetalTarget) Run(ctx context.Context, p Process) error {
	return t.ExecToStdout(ctx, p.Command, p.Directory)
}

func (t *MetalTarget) SetEnv(key string, value string) error {
	return os.Setenv(key, value)
}
func (t *MetalTarget) MkdirAsNeeded(ctx context.Context, path string) error {
	return plumbing.MkdirAsNeeded(path)
}

func (t *MetalTarget) ExecToStdoutWithExtraEnv(ctx context.Context, cmdString string, targetDir string, env []string) error {
	env = append(os.Environ(), env...)
	return t.ExecToStdoutWithEnv(ctx, cmdString, targetDir, env)
}

func (t *MetalTarget) ExecToStdoutWithEnv(ctx context.Context, cmdString string, targetDir string, env []string) error {
	log.Infof("Running: %s in %s", cmdString, targetDir)
	cmdArgs, err := shlex.Split(cmdString)
	if err != nil {
		return fmt.Errorf("Can't parse command string '%s': %v", cmdString, err)
	}

	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:len(cmdArgs)]...)
	cmd.Dir = targetDir
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stdout
	cmd.Env = env

	log.Debugf("Process env: %v", env)

	err = cmd.Run()

	if err != nil {
		return fmt.Errorf("Command failed to run with error: %v", err)
	}

	return nil
}

func (t *MetalTarget) ExecToStdout(ctx context.Context, cmdString string, targetDir string) error {
	return t.ExecToStdoutWithEnv(ctx, cmdString, targetDir, os.Environ())
}

func (t *MetalTarget) ExecToLog(ctx context.Context, cmdString string, targetDir string, logPath string) error {

	cmdArgs, err := shlex.Split(cmdString)
	if err != nil {
		return fmt.Errorf("Can't parse command string '%s': %v", cmdString, err)
	}

	logfile, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("Couldn't open log file %s: %v", logPath, err)
	}

	defer logfile.Close()

	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:len(cmdArgs)]...)
	cmd.Dir = targetDir
	cmd.Stdout = logfile
	cmd.Stdin = os.Stdin
	cmd.Stderr = logfile

	err = cmd.Run()

	if err != nil {
		return fmt.Errorf("Command '%s' failed to run with error -- see log for information: %s", cmdString, logPath)
	}

	return nil

}

func (t *MetalTarget) ExecSilently(ctx context.Context, cmdString string, targetDir string) error {
	return t.ExecSilentlyToWriter(ctx, cmdString, targetDir, ioutil.Discard)
}

func (t *MetalTarget) ExecSilentlyToWriter(ctx context.Context, cmdString string, targetDir string, writer io.Writer) error {
	cmdArgs, err := shlex.Split(cmdString)
	if err != nil {
		return fmt.Errorf("Can't parse command string '%s': %v", cmdString, err)
	}

	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:len(cmdArgs)]...)
	cmd.Dir = targetDir
	cmd.Stdout = writer
	cmd.Stdin = os.Stdin
	cmd.Stderr = writer

	err = cmd.Run()

	if err != nil {
		return fmt.Errorf("Command '%s' failed to run with error %v", cmdString, err)
	}

	return nil

}

func (t *MetalTarget) ExecToLogWithProgressDots(ctx context.Context, cmdString string, targetDir string, logPath string) error {
	stoppedchan := make(chan struct{})
	dotchan := make(chan int)
	defer close(stoppedchan)

	go func() {
		for {
			select {
			default:
				dotchan <- 1
				time.Sleep(3 * time.Second)
			case <-stoppedchan:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			default:
			case <-dotchan:
				fmt.Printf(".")
			case <-stoppedchan:
				fmt.Printf(" done!\n")
				return
			}
		}
	}()

	return t.ExecToLog(ctx, cmdString, targetDir, logPath)
}
func (t *MetalTarget) WriteFileContents(ctx context.Context, contents string, path string) error {
	return ioutil.WriteFile(path, []byte(contents), 0644)
}
