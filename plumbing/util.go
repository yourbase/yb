package plumbing

import (
	"bytes"
	"errors"
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
		return fmt.Errorf("compressing data: %s\n", err)
	}

	if _, err := io.Copy(xzWriter, b); err != nil {
		return fmt.Errorf("compressing data: %s\n", err)
	}
	xzWriter.Close()

	b.Reset()
	b.Write(buf.Bytes())

	return nil
}

func DecompressBuffer(b *bytes.Buffer) error {
	xzReader, err := xz.NewReader(b)

	if err != nil {
		return fmt.Errorf("decompressing data: %s\n", err)
	}

	var buf bytes.Buffer

	if _, err := io.Copy(&buf, xzReader); err != nil {
		return fmt.Errorf("decompressing data: %v", err)
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
		return nil, fmt.Errorf("define a branch to clone repo %v", remote.Url)
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
		err = fmt.Errorf("searching for a SSH agent/key configuration")
	}

	return
}

// IsBinary returns whether a file contains a NUL byte near the beginning of the file.
func IsBinary(filePath string) (bool, error) {
	r, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer r.Close()

	buf := make([]byte, 8000)
	n, err := io.ReadFull(r, buf)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			err = nil
		} else {
			err = fmt.Errorf("check for binary: %w", err)
		}
		return false, 
	}
	for _, b := range buf[:n] {
		if b == 0 {
			return true, err
		}
	}
	return false, err
}
