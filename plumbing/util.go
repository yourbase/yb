package plumbing

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/google/shlex"
	"github.com/ulikunitz/xz"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/types"
	"gopkg.in/src-d/go-billy.v4/memfs"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	githttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

const SNIFFLEN = 8000

func ExecToStdoutWithExtraEnv(cmdString string, targetDir string, env []string) error {
	env = append(os.Environ(), env...)
	return ExecToStdoutWithEnv(cmdString, targetDir, env)
}

func ExecToStdoutWithEnv(cmdString string, targetDir string, env []string) error {
	log.Infof("Running: %s in %s", cmdString, targetDir)
	cmdArgs, err := shlex.Split(cmdString)
	if err != nil {
		return fmt.Errorf("Can't parse command string '%s': %v", cmdString, err)
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = targetDir
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stdout
	cmd.Env = env

	err = cmd.Run()

	if err != nil {
		return fmt.Errorf("Command failed to run with error: %v\n", err)
	}

	return nil
}

func ExecToStdout(cmdString string, targetDir string) error {
	return ExecToStdoutWithEnv(cmdString, targetDir, os.Environ())
}

func ExecToLog(cmdString string, targetDir string, logPath string) error {
	cmdArgs, err := shlex.Split(cmdString)
	if err != nil {
		return fmt.Errorf("Can't parse command string '%s': %v", cmdString, err)
	}

	logfile, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("Couldn't open log file %s: %v", logPath, err)
	}

	defer logfile.Close()

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
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

func ExecSilently(cmdString string, targetDir string) error {
	return ExecSilentlyToWriter(cmdString, targetDir, ioutil.Discard)
}
func ExecSilentlyToWriter(cmdString string, targetDir string, writer io.Writer) error {
	cmdArgs, err := shlex.Split(cmdString)
	if err != nil {
		return fmt.Errorf("Can't parse command string '%s': %v", cmdString, err)
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
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

func ExecToLogWithProgressDots(cmdString string, targetDir string, logPath string) error {
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

	return ExecToLog(cmdString, targetDir, logPath)
}

func PrependToPath(dir string) {
	currentPath := os.Getenv("PATH")
	// Only prepend if it's not already the head; presume that
	// whomever asked for this wants to be at the front so it's okay if it's
	// duplicated later
	if !strings.HasPrefix(currentPath, dir) {
		newPath := fmt.Sprintf("%s:%s", dir, currentPath)
		os.Setenv("PATH", newPath)
	}
}

func ConfigFilePath(filename string) string {
	u, _ := user.Current()
	configDir := filepath.Join(u.HomeDir, ".config", "yb")
	MkdirAsNeeded(configDir)
	filePath := filepath.Join(configDir, filename)
	return filePath
}

func PathExists(path string) bool {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return false
	}

	return true
}

func DirectoryExists(dir string) bool {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false
	}

	return true
}

func MkdirAsNeeded(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Infof("Making dir: %s", dir)
		if err := os.MkdirAll(dir, 0700); err != nil {
			log.Errorf("Unable to create dir: %v", err)
			return err
		}
	}

	return nil
}

func TemplateToString(templateText string, data interface{}) (string, error) {
	t, err := template.New("generic").Parse(templateText)
	if err != nil {
		return "", err
	}
	var tpl bytes.Buffer
	if err := t.Execute(&tpl, data); err != nil {
		log.Errorf("Can't render template:: %v", err)
		return "", err
	}

	result := tpl.String()
	return result, nil
}

func RemoveWritePermission(path string) bool {
	info, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false
	}

	//Check 'others' permission
	p := info.Mode()
	newmask := p & (0555)
	os.Chmod(path, newmask)

	return true
}

func RemoveWritePermissionRecursively(path string) bool {
	fileList := []string{}

	err := filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		fileList = append(fileList, path)
		return nil
	})

	if err != nil {
		return false
	}

	for _, file := range fileList {
		RemoveWritePermission(file)
	}

	return true
}

func ToolsDir() string {
	toolsDir, exists := os.LookupEnv("YB_TOOLS_DIR")
	if !exists {
		u, err := user.Current()
		if err != nil {
			toolsDir = "/tmp/yourbase/tools"
		} else {
			toolsDir = fmt.Sprintf("%s/.yourbase/tools", u.HomeDir)
		}
	}

	MkdirAsNeeded(toolsDir)

	return toolsDir
}

func CacheDir() string {
	cacheDir, exists := os.LookupEnv("YB_CACHE_DIR")
	if !exists {
		u, err := user.Current()
		if err != nil {
			cacheDir = "/tmp/yourbase/cache"
		} else {
			cacheDir = fmt.Sprintf("%s/.yourbase/cache", u.HomeDir)
		}
	}

	MkdirAsNeeded(cacheDir)

	return cacheDir
}

func CacheFilenameForUrl(url string) (string, error) {
	cacheDir := CacheDir()
	reg, err := regexp.Compile("[^a-zA-Z0-9.]+")
	if err != nil {
		return "", fmt.Errorf("Can't compile regex: %v", err)
	}

	fileName := reg.ReplaceAllString(url, "")
	return filepath.Join(cacheDir, fileName), nil
}

func DownloadFileWithCache(url string) (string, error) {
	cacheFilename, err := CacheFilenameForUrl(url)

	if err != nil {
		return cacheFilename, err
	}

	fileExists := false
	fileSizeMismatch := false

	// Exists, don't re-download
	if fi, err := os.Stat(cacheFilename); !os.IsNotExist(err) {
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
	err = DownloadFile(cacheFilename, url)
	return cacheFilename, err
}

func DownloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
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

/*
 * Look in the directory above the manifest file, if there's a config.yml, use that
 * otherwise we use the directory of the manifest file as the workspace root
 */
func FindWorkspaceRoot() (string, error) {
	wd, err := os.Getwd()

	if err != nil {
		panic(err)
	}

	if _, err := os.Stat(filepath.Join(wd, "config.yml")); err == nil {
		// If we're currently in the directory with the config.yml
		return wd, nil
	}

	// Look upwards to find a manifest file
	packageDir, err := FindNearestManifestFile()

	// If we find a manifest file, check the parent directory for a config.yml
	if err == nil {
		parent := filepath.Dir(packageDir)
		if _, err := os.Stat(filepath.Join(parent, "config.yml")); err == nil {
			return parent, nil
		}
	} else {
		return "", err
	}

	// No config in the parent of the package? No workspace!
	return "", fmt.Errorf("No workspace root found nearby.")
}

func FindFileUpTree(filename string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	for {
		file_path := filepath.Join(wd, filename)
		if _, err := os.Stat(file_path); err == nil {
			return wd, nil
		}

		wd = filepath.Dir(wd)

		if strings.HasSuffix(wd, "/") {
			return "", fmt.Errorf("Can't find %s, ended up at the root...", filename)
		}
	}
}

func FindNearestManifestFile() (string, error) {
	return FindFileUpTree(types.MANIFEST_FILE)
}

// Because, why not?
// Based on https://github.com/sindresorhus/is-docker/blob/master/index.js and https://github.com/moby/moby/issues/18355
// Discussion is not settled yet: https://stackoverflow.com/questions/23513045/how-to-check-if-a-process-is-running-inside-docker-container#25518538
func InsideTheMatrix() bool {
	hasDockerEnv := PathExists("/.dockerenv")
	hasDockerCGroup := false
	dockerCGroupPath := "/proc/self/cgroup"
	if PathExists(dockerCGroupPath) {
		contents, _ := ioutil.ReadFile(dockerCGroupPath)
		hasDockerCGroup = strings.Count(string(contents), "docker") > 0
	}
	return hasDockerEnv || hasDockerCGroup
}

func CompressBuffer(b *bytes.Buffer) error {
	var buf bytes.Buffer

	xzWriter, err := xz.NewWriter(&buf)
	if err != nil {
		return fmt.Errorf("Unable to compress data: %s\n", err)
	}

	if _, err := io.Copy(xzWriter, b); err != nil {
		return fmt.Errorf("Unable to compress data: %s\n", err)
	}
	xzWriter.Close()

	b.Reset()
	b.Write(buf.Bytes())

	return nil
}

func DecompressBuffer(b *bytes.Buffer) error {
	xzReader, err := xz.NewReader(b)

	if err != nil {
		return fmt.Errorf("Unable to decompress data: %s\n", err)
	}

	var buf bytes.Buffer

	if _, err := io.Copy(&buf, xzReader); err != nil {
		return fmt.Errorf("Unable to decompress data: %v", err)
	}

	b.Reset()
	b.Write(buf.Bytes())

	return nil
}

func CloneRepository(remote types.GitRemote, inMem bool, basePath string) (rep *git.Repository, err error) {
	if remote.Branch == "" {
		return nil, fmt.Errorf("No branch defined to clone repo %v", remote.Url)
	}

	cloneOpts := &git.CloneOptions{
		URL:           remote.String(),
		ReferenceName: plumbing.NewBranchReferenceName(remote.Branch),
		SingleBranch:  true,
		Depth:         50,
		Tags:          git.NoTags,
	}

	if remote.Token != "" {
		cloneOpts.Auth = &githttp.BasicAuth{
			Username: remote.User,
			Password: remote.Token,
		}
	} else if remote.Password != "" || remote.User != "" {
		cloneOpts.Auth = &githttp.BasicAuth{
			Username: remote.User,
			Password: remote.Password,
		}
	}

	if inMem {
		fs := memfs.New()
		storer := memory.NewStorage()

		rep, err = git.Clone(storer, fs, cloneOpts)
	} else {
		rep, err = git.PlainClone(basePath, false, cloneOpts)
	}
	if err != nil && strings.Count(err.Error(), "SSH") > 0 {
		err = fmt.Errorf("Please check your SSH agent/key configuration")
	}

	return
}

// IsBinary detects if data is a binary value based on:
// http://git.kernel.org/cgit/git/git.git/tree/xdiff-interface.c?id=HEAD#n198
func IsBinary(filePath string) (bool, error) {
	r, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer r.Close()

	reader := bufio.NewReader(r)
	c := 0
	for {
		if c == SNIFFLEN {
			break
		}

		b, err := reader.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}

		if b == byte(0) {
			return true, nil
		}

		c++
	}

	return false, nil
}
