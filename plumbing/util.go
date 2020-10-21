package plumbing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/google/shlex"
	"github.com/ulikunitz/xz"
	"zombiezen.com/go/log"
)

func ExecToStdoutWithEnv(cmdString string, targetDir string, env []string) error {
	log.Infof(context.TODO(), "Running: %s in %s", cmdString, targetDir)
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

func TemplateToString(templateText string, data interface{}) (string, error) {
	t, err := template.New("generic").Parse(templateText)
	if err != nil {
		return "", err
	}
	var tpl bytes.Buffer
	if err := t.Execute(&tpl, data); err != nil {
		log.Errorf(context.TODO(), "Can't render template:: %v", err)
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
		// Ignore EOF, since it's fine for the file to be shorter than the buffer size.
		// Otherwise, wrap the error. We don't fully stop the control flow here because
		// we may still have read enough data to make a determination.
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			err = nil
		} else {
			err = fmt.Errorf("check for binary: %w", err)
		}
	}
	for _, b := range buf[:n] {
		if b == 0 {
			return true, err
		}
	}
	return false, err
}
