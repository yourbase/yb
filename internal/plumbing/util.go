package plumbing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	"github.com/ulikunitz/xz"
	"zombiezen.com/go/log"
)

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

// Because, why not?
// Based on https://github.com/sindresorhus/is-docker/blob/master/index.js and https://github.com/moby/moby/issues/18355
// Discussion is not settled yet: https://stackoverflow.com/questions/23513045/how-to-check-if-a-process-is-running-inside-docker-container#25518538
func InsideTheMatrix() bool {
	hasDockerEnv := pathExists("/.dockerenv")
	hasDockerCGroup := false
	dockerCGroupPath := "/proc/self/cgroup"
	if pathExists(dockerCGroupPath) {
		contents, _ := ioutil.ReadFile(dockerCGroupPath)
		hasDockerCGroup = strings.Count(string(contents), "docker") > 0
	}
	return hasDockerEnv || hasDockerCGroup
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return !os.IsNotExist(err)
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
