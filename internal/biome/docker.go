// Copyright 2020 YourBase Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package biome

import (
	"archive/tar"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	slashpath "path"
	"path/filepath"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/yourbase/commons/xcontext"
	"github.com/yourbase/narwhal"
	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/ybtrace"
	"go.opentelemetry.io/otel/codes"
	"zombiezen.com/go/log"
)

// BindMount is the docker.HostMount.Type for a bind mount.
const BindMount = "bind"

// Container is a biome that executes processes inside a Docker container.
type Container struct {
	client *docker.Client
	desc   Descriptor
	id     string
	path   string
	dirs   Dirs
}

const (
	TiniURL    = "https://github.com/krallin/tini/releases/download/v0.19.0/tini-amd64"
	tiniSize   = 24064
	tiniSHA256 = "93dcc18adc78c65a028a84799ecf8ad40c936fdfc5f2a57b1acda5a8117fa82c"
	tiniPath   = "/tini"
)

// ContainerOptions holds parameters for CreateContainer.
type ContainerOptions struct {
	// PackageDir is the path on the host where the package is located.
	// Must not be empty and the directory must exist.
	PackageDir string
	// HomeDir is the path on the host to use as the biome's home directory.
	// Must not be empty and the directory must exist.
	HomeDir string
	// TiniExe is a stream of TiniURL's contents. Must be non-nil.
	TiniExe io.Reader

	// PullOutput is where any `docker pull` progress output is sent to.
	// If nil, this output is discarded.
	PullOutput io.Writer
	// Definition optionally specifies details about the container to create.
	Definition *narwhal.ContainerDefinition
	// NetworkID is a Docker network ID to connect the container to, if not empty.
	NetworkID string
}

const containerHome = "/home/yourbase"

func (opts *ContainerOptions) definition() (*narwhal.ContainerDefinition, error) {
	defn := new(narwhal.ContainerDefinition)
	if opts.Definition != nil {
		*defn = *opts.Definition
	}
	// TODO(light): Randomness is necessary because Narwhal names are not unique
	// by default.
	var bits [4]byte
	if _, err := rand.Read(bits[:]); err != nil {
		return nil, err
	}
	defn.Namespace = hex.EncodeToString(bits[:])
	if defn.Image == "" {
		defn.Image = yb.DefaultContainerImage
	}
	if defn.WorkDir == "" {
		defn.WorkDir = "/workspace"
	}
	defn.Mounts = append(defn.Mounts[:len(defn.Mounts):len(defn.Mounts)],
		docker.HostMount{
			Source: opts.PackageDir,
			Target: defn.WorkDir,
			Type:   BindMount,
		},
		docker.HostMount{
			Source: opts.HomeDir,
			Target: containerHome,
			Type:   BindMount,
		},
	)
	defn.Argv = []string{tiniPath, "-g", "--", "tail", "-f", "/dev/null"}
	return defn, nil
}

// CreateContainer starts a new Docker container. It is the caller's
// responsibility to call Close on the container.
func CreateContainer(ctx context.Context, client *docker.Client, opts *ContainerOptions) (_ *Container, err error) {
	ctx, span := ybtrace.Start(ctx, "Create Container")
	defer func() {
		if err != nil {
			span.SetStatus(codes.Unknown, err.Error())
		}
		span.End()
	}()

	desc, err := DockerDescriptor(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("create build container: %w", err)
	}
	defn, err := opts.definition()
	if err != nil {
		return nil, fmt.Errorf("create build container: %w", err)
	}
	span.SetAttribute("docker.image", defn.Image)
	for _, mount := range defn.Mounts {
		if mount.Type != BindMount {
			continue
		}
		if err := makeMount(mount.Source); err != nil {
			return nil, fmt.Errorf("create build container: %w", err)
		}
	}
	pullOutput := opts.PullOutput
	if pullOutput == nil {
		pullOutput = ioutil.Discard
	}
	log.Infof(ctx, "Creating %s container...", defn.Image)
	containerID, err := narwhal.CreateContainer(ctx, client, pullOutput, defn)
	if err != nil {
		return nil, fmt.Errorf("create build container: %w", err)
	}
	span.SetAttribute("docker.container_id", containerID)
	defer func() {
		if err != nil {
			rmErr := client.RemoveContainer(docker.RemoveContainerOptions{
				Context: xcontext.IgnoreDeadline(ctx),
				ID:      containerID,
				Force:   true,
			})
			if rmErr != nil {
				log.Warnf(ctx, "Cleaning up container %s: %v", containerID, rmErr)
			}
		}
	}()

	if opts.NetworkID != "" {
		err := client.ConnectNetwork(opts.NetworkID, docker.NetworkConnectionOptions{
			Context:   ctx,
			Container: containerID,
			EndpointConfig: &docker.EndpointConfig{
				NetworkID: opts.NetworkID,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("create build container: %w", err)
		}
	}
	if err := uploadTini(ctx, client, containerID, opts.TiniExe); err != nil {
		return nil, fmt.Errorf("create build container: %w", err)
	}
	if err := narwhal.MkdirAll(ctx, client, containerID, containerHome, nil); err != nil {
		return nil, fmt.Errorf("create build container: create home: %w", err)
	}
	if err := narwhal.StartContainer(ctx, client, containerID, defn.HealthCheckPort); err != nil {
		return nil, fmt.Errorf("create build container: %w", err)
	}
	return &Container{
		client: client,
		desc:   *desc,
		id:     containerID,
		dirs: Dirs{
			Home:    containerHome,
			Package: defn.WorkDir,
			Tools:   containerHome + "/.cache/yb/tools",
		},
		// TODO(light): Probe container for PATH.
		path: "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}, nil
}

// makeMount ensures that the given path on the host exists for mounting,
// creating a directory as needed.
func makeMount(hostPath string) error {
	if _, err := os.Stat(hostPath); err == nil {
		// If the path already exists (regardless of directory or file), that's all
		// we need.
		return nil
	}
	return os.MkdirAll(hostPath, 0o777)
}

func uploadTini(ctx context.Context, client *docker.Client, containerID string, tiniExe io.Reader) error {
	sha256Hash := sha256.New()
	tiniHeader := &tar.Header{
		Size:     tiniSize,
		Typeflag: tar.TypeReg,
		Mode:     0777,
	}
	err := narwhal.Upload(ctx, client, containerID, tiniPath, io.TeeReader(tiniExe, sha256Hash), tiniHeader)
	if err != nil {
		return fmt.Errorf("upload tini: %w", err)
	}
	if hex.EncodeToString(sha256Hash.Sum(nil)) != tiniSHA256 {
		return fmt.Errorf("upload tini: checksum mismatch")
	}
	return nil
}

// Close stops and removes the container.
func (c *Container) Close() error {
	return c.client.RemoveContainer(docker.RemoveContainerOptions{
		Context: context.Background(),
		ID:      c.id,
		Force:   true,
	})
}

// Describe returns the Docker daemon's operating system and architecture
// information.
func (c *Container) Describe() *Descriptor {
	return &c.desc
}

// DockerDescriptor returns a descriptor for Docker daemon's container
// environment.
func DockerDescriptor(ctx context.Context, client *docker.Client) (*Descriptor, error) {
	// TODO(someday): There's no method that accepts Context.
	info, err := client.Info()
	if err != nil {
		return nil, fmt.Errorf("docker info: %w", err)
	}
	if info.OSType == "" || info.Architecture == "" {
		return nil, fmt.Errorf("docker info: missing OSType and/or Architecture")
	}
	desc := &Descriptor{
		OS: info.OSType,
		// While the Docker documentation claims that it uses runtime.GOARCH,
		// it actually uses the syscall equivalent of `uname -m`.
		// Source: https://github.com/moby/moby/blob/v20.10.7/pkg/platform/architecture_unix.go
		Arch: map[string]string{
			"x86":    Intel32,
			"x86_64": Intel64,
		}[info.Architecture],
	}
	if desc.Arch == "" {
		return nil, fmt.Errorf("docker info: unknown architecture %q", info.Architecture)
	}
	return desc, nil
}

// Dirs returns special directories.
func (c *Container) Dirs() *Dirs {
	return &c.dirs
}

// Run runs a process in the container and waits for it to exit.
func (c *Container) Run(ctx context.Context, invoke *Invocation) error {
	if len(invoke.Argv) == 0 {
		return fmt.Errorf("run in container %s: argv empty", c.id)
	}

	log.Debugf(ctx, "Run (Docker): %s", strings.Join(invoke.Argv, " "))
	log.Debugf(ctx, "Running in container %s", c.id)
	log.Debugf(ctx, "Environment:\n%v", invoke.Env)

	opts := docker.CreateExecOptions{
		Context:      ctx,
		Container:    c.id,
		Cmd:          append([]string{"env", "--"}, invoke.Argv...),
		AttachStdin:  invoke.Stdin != nil,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          invoke.Interactive,
	}
	opts.Env = []string{
		// TODO(light): Set LOGNAME and USER.
		"HOME=" + c.dirs.Home,
	}
	if v, ok := os.LookupEnv("NO_COLOR"); ok {
		opts.Env = append(opts.Env, "NO_COLOR="+v)
	}
	opts.Env = appendStandardEnv(opts.Env, c.Describe().OS)
	opts.Env = invoke.Env.appendTo(opts.Env, c.path, ':')
	if slashpath.IsAbs(invoke.Dir) {
		opts.WorkingDir = invoke.Dir
	} else {
		opts.WorkingDir = filepath.Join(c.dirs.Package, invoke.Dir)
	}
	exec, err := c.client.CreateExec(opts)
	if err != nil {
		return fmt.Errorf("run in container %s: %w", c.id, err)
	}

	// Unfortunately, there's no way to signal a particular exec:
	// https://github.com/moby/moby/issues/9098
	//
	// So if the Context is cancelled, we restart the container, which may take
	// down other running processes, but usually the biome is going down anyway.
	runDone := make(chan struct{})
	interrupterDone := make(chan struct{})
	go func() {
		defer close(interrupterDone)
		select {
		case <-ctx.Done():
			log.Infof(ctx, "Interrupted. Stopping container %s...", c.id)
			if err := c.client.StopContainer(c.id, 10); err != nil {
				log.Warnf(ctx, "Could not stop container %s: %v", c.id, err)
				return
			}
			err := c.client.StartContainerWithContext(
				c.id,
				&docker.HostConfig{},
				xcontext.IgnoreDeadline(ctx),
			)
			if err != nil {
				log.Warnf(ctx, "Could not restart container %s after interrupt: %v", c.id, err)
				return
			}
		case <-runDone:
		}
	}()

	stdout := ioutil.Discard
	if invoke.Stdout != nil {
		stdout = invoke.Stdout
	}
	stderr := ioutil.Discard
	if invoke.Stderr != nil {
		stderr = invoke.Stderr
	}
	err = c.client.StartExec(exec.ID, docker.StartExecOptions{
		Context:      ctx,
		InputStream:  invoke.Stdin,
		OutputStream: stdout,
		ErrorStream:  stderr,
		// TODO(light): Tty can't be set true or it fails with "Unrecognized input header: 114".
	})
	close(runDone)
	<-interrupterDone
	if err != nil {
		return fmt.Errorf("run in container %s: %w", c.id, err)
	}
	results, err := c.client.InspectExec(exec.ID)
	if err != nil {
		return fmt.Errorf("run in container %s: %w", c.id, err)
	}
	if results.ExitCode != 0 {
		return fmt.Errorf("run in container %s: exit code %d", c.id, results.ExitCode)
	}
	return nil
}

// WriteFile writes a file to the given path in the container.
func (c *Container) WriteFile(ctx context.Context, path string, src io.Reader) error {
	if seeker, ok := src.(io.Seeker); ok {
		// If the reader can seek, then we can find out the file size without
		// reading the whole file.
		if size, err := seeker.Seek(0, io.SeekEnd); err != nil {
			log.Debugf(ctx, "Seeking in reader used for Container.WriteFile failed: %v", err)
		} else {
			if _, err := seeker.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("write file %s: %w", path, err)
			}
			return narwhal.Write(ctx, c.client, c.id, path, src, &tar.Header{
				Typeflag: tar.TypeReg,
				Size:     size,
				Mode:     0644,
			})
		}
	}
	f, err := ioutil.TempFile("", "yb_container_file")
	if err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}
	defer func() {
		name := f.Name()
		f.Close()
		if err := os.Remove(name); err != nil {
			log.Warnf(ctx, "Cleaning up temporary file: %v", err)
		}
	}()
	size, err := io.Copy(f, src)
	if err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}
	return narwhal.Write(ctx, c.client, c.id, path, src, &tar.Header{
		Typeflag: tar.TypeReg,
		Size:     size,
		Mode:     0644,
	})
}

// MkdirAll ensures the given directory and any parent directories exist in
// the container.
func (c *Container) MkdirAll(ctx context.Context, path string) error {
	return narwhal.MkdirAll(ctx, c.client, c.id, path, nil)
}

// JoinPath calls path.Join.
func (c *Container) JoinPath(elem ...string) string {
	return slashpath.Join(elem...)
}

// IsAbsPath calls path.IsAbs.
func (c *Container) IsAbsPath(path string) bool {
	return slashpath.IsAbs(path)
}
