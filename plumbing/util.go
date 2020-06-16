package plumbing

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/types"

	"github.com/ulikunitz/xz"

	"gopkg.in/src-d/go-billy.v4/memfs"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	githttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

const SNIFFLEN = 8000

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

/*
func PrintDownloadPercent(done chan int64, path string, total int64) {

	var stop bool = false

	for {
		select {
		case <-done:
			stop = true
		default:

			file, err := os.Open(path)
			if err != nil {
				log.Fatal(err)
			}

			fi, err := file.Stat()
			if err != nil {
				log.Fatal(err)
			}

			size := fi.Size()

			if size == 0 {
				size = 1
			}

			var percent float64 = float64(size) / float64(total) * 100

			fmt.Printf("%.0f", percent)
			fmt.Println("%")
		}

		if stop {
			break
		}

		time.Sleep(time.Second)
	}
}

func DownloadFile(dest string, url string) error {

	log.Printf("Downloading file %s from %s\n", dest, url)

	start := time.Now()

	out, err := os.Create(dest)

	if err != nil {
		return fmt.Errorf("Unable to open destination '%s': %v\n", dest, err)
	}

	defer out.Close()

	headResp, err := http.Head(url)

	if err != nil {
		panic(err)
	}

	defer headResp.Body.Close()

	size, err := strconv.Atoi(headResp.Header.Get("Content-Length"))

	if err != nil {
		panic(err)
	}

	done := make(chan int64)

	go PrintDownloadPercent(done, dest, int64(size))

	resp, err := http.Get(url)

	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	n, err := io.Copy(out, resp.Body)

	if err != nil {
		panic(err)
	}

	done <- n

	elapsed := time.Since(start)
	log.Printf("Download completed in %s", elapsed)
	return nil
} */

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

	fmt.Print("WorkspaceRoot considered: ", packageDir)

	// If we find a manifest file, check the parent directory for a config.yml
	if err == nil {
		parent := filepath.Dir(packageDir)
		if _, err := os.Stat(filepath.Join(parent, "config.yml")); err == nil {
			return parent, nil
		} else {
			return packageDir, nil
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
	return FindFileUpTree(MANIFEST_FILE)
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

// Returns two empty strings and false if env isn't formed as "something=.*"
// Or else return env name, value and true
func SaneEnvironmentVar(env string) (name, value string, sane bool) {
	s := strings.SplitN(env, "=", 2)
	if sane = (len(s) == 2); sane {
		name = s[0]
		value = s[1]
	}
	return
}

func CloneRepository(remote GitRemote, inMem bool, basePath string) (rep *git.Repository, err error) {
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
