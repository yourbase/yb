package runtime

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/google/shlex"
	"github.com/johnewart/archiver"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
)

type MetalTarget struct {
	Target
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

// TODO: Add support for the OS's here
func (t *MetalTarget) OSVersion(ctx context.Context) string {
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

func (t *MetalTarget) CacheDir(ctx context.Context) string {
	cacheDir, exists := os.LookupEnv("YB_CACHE_DIR")
	if !exists {
		u, err := user.Current()
		if err != nil {
			cacheDir = "/tmp/yourbase/cache"
		} else {
			cacheDir = fmt.Sprintf("%s/.yourbase/cache", u.HomeDir)
		}
	}

	plumbing.MkdirAsNeeded(cacheDir)

	return cacheDir
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

	localFile, err := DownloadFileToCache(ctx, url, t.CacheDir(ctx))

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

func CacheFilenameForUrl(url string) (string, error) {
	reg, err := regexp.Compile("[^a-zA-Z0-9.]+")
	if err != nil {
		return "", fmt.Errorf("Can't compile regex: %v", err)
	}
	fileName := reg.ReplaceAllString(url, "")
	return fileName, nil
}

func DownloadFileWithCache(ctx context.Context, url string) (string, error) {
	cacheDir := "/tmp/yourbase"
	if homeDir, exists := os.LookupEnv("HOME"); exists {
		cacheDir = filepath.Join(homeDir, ".cache", "yourbase")
	}

	return DownloadFileToCache(ctx, url, cacheDir)
}

func DownloadFileToCache(ctx context.Context, url string, cachedir string) (string, error) {

	filename, err := CacheFilenameForUrl(url)
	if err != nil {
		return "", err
	}

	os.MkdirAll(cachedir, 0700)

	cacheFilename := filepath.Join(cachedir, filename)
	log.Infof("Downloading %s to cache as %s", url, cacheFilename)

	fileExists := false
	fileSizeMismatch := false

	// Exists, don't re-download
	if fi, err := os.Stat(cacheFilename); !os.IsNotExist(err) && fi != nil {
		fileExists = true

		// try HEAD'ing the URL and comparing to local file
		resp, err := http.Head(url)
		if err == nil {
			if fi.Size() != resp.ContentLength {
				log.Infof("Re-downloading %s because remote file and local file differ in size", url)
				fileSizeMismatch = true
			}
		}

	}

	if fileExists && !fileSizeMismatch {
		// No mismatch known, but exists, use cached version
		log.Infof("Re-using cached version of %s", url)
		return cacheFilename, nil
	}

	// Otherwise download
	err = doDownload(ctx, cacheFilename, url)
	return cacheFilename, err
}

func doDownload(ctx context.Context, filepath string, url string) error {

	// Cancellable request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	client := &http.Client{}

	// Get the data
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}
