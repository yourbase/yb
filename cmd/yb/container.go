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

package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	slashpath "path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/yourbase/commons/ini"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/ybdata"
	"golang.org/x/sys/unix"
	"zombiezen.com/go/log"
)

func newContainerBiome(ctx context.Context, dataDirs *ybdata.Dirs, packageDir, homeDir, image string) (*containerBiome, error) {
	creds, err := ini.ParseFiles(nil, "docker.cfg")
	if err != nil {
		return nil, fmt.Errorf("new container: %w", err)
	}

	username := creds.Get("", "username")
	password := creds.Get("", "password")
	log.Infof(ctx, "Logging in as %s", username)
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
	layers, err := pullImage(ctx, dataDirs, image, auth)
	if err != nil {
		return nil, fmt.Errorf("new container: %w", err)
	}

	scratchDir, err := ioutil.TempDir("", "yb-scratch-*")
	if err != nil {
		return nil, fmt.Errorf("new container: %w", err)
	}
	return &containerBiome{
		packageDir: packageDir,
		homeDir:    homeDir,
		layers:     layers,
		scratchDir: scratchDir,
	}, nil
}

type containerBiome struct {
	packageDir string
	homeDir    string
	layers     []string
	scratchDir string
}

func (cb containerBiome) Close() error {
	return os.RemoveAll(cb.scratchDir)
}

// Describe returns the values of GOOS/GOARCH.
func (cb containerBiome) Describe() *biome.Descriptor {
	return &biome.Descriptor{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}
}

const (
	containerPackageMount = "/workspace"
	containerHome         = "/root"
)

// Dirs returns special directories.
func (cb containerBiome) Dirs() *biome.Dirs {
	return &biome.Dirs{
		Package: containerPackageMount,
		Home:    containerHome,
		Tools:   containerHome + "/.cache/yb/tools",
	}
}

// Run runs a subprocess and waits for it to exit.
func (cb containerBiome) Run(ctx context.Context, invoke *biome.Invocation) error {
	if len(invoke.Argv) == 0 {
		return fmt.Errorf("container run: argv empty")
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("container run: %w", err)
	}
	log.Infof(ctx, "Run: %s", strings.Join(invoke.Argv, " "))
	log.Debugf(ctx, "Environment:\n%v", invoke.Env)
	args := []string{
		"container-init",
		"--scratch=" + cb.scratchDir,
		"--mount=" + containerPackageMount + "=" + cb.packageDir,
		"--mount=" + containerHome + "=" + cb.homeDir,
	}
	for _, layer := range cb.layers {
		args = append(args, "--root="+layer)
	}
	if slashpath.IsAbs(invoke.Dir) {
		args = append(args, "--workdir="+invoke.Dir)
	} else {
		args = append(args, "--workdir="+slashpath.Join(containerPackageMount, invoke.Dir))
	}
	args = append(args, "--")
	args = append(args, invoke.Argv...)
	c := exec.CommandContext(ctx, exe, args...)
	c.Env = []string{
		"HOME=/root",
		"LOGNAME=root",
		"USER=root",
		"TZ=UTC",
	}
	// TODO(light): Probe image for PATH.
	const containerPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	c.Env = invoke.Env.AppendTo(c.Env, containerPath, ':')
	c.SysProcAttr = &unix.SysProcAttr{
		Cloneflags: unix.CLONE_NEWNS |
			unix.CLONE_NEWPID |
			unix.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
			// {
			// 	ContainerID: 1,
			// 	HostID:      165536,
			// 	Size:        65536,
			// },
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
			// {
			// 	ContainerID: 1,
			// 	HostID:      165536,
			// 	Size:        65536,
			// },
		},
	}
	c.Stdin = invoke.Stdin
	c.Stdout = invoke.Stdout
	c.Stderr = invoke.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("container run: %w", err)
	}
	return nil
}

// JoinPath calls path.Join.
func (cb containerBiome) JoinPath(elem ...string) string {
	return slashpath.Join(elem...)
}

// CleanPath calls path.Clean.
func (cb containerBiome) CleanPath(path string) string {
	return slashpath.Clean(path)
}

// IsAbsPath calls path.IsAbs.
func (cb containerBiome) IsAbsPath(path string) bool {
	return slashpath.IsAbs(path)
}

type containerInitCmd struct {
	rootLayers []string
	mounts     map[string]string
	scratchDir string
	chdir      string
}

func newContainerInitCmd() *cobra.Command {
	p := new(containerInitCmd)
	c := &cobra.Command{
		Use:           "container-init",
		Hidden:        true,
		Args:          cobra.MinimumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return p.run(cmd.Context(), args)
		},
	}
	c.Flags().StringSliceVar(&p.rootLayers, "root", nil, "path to root filesystem (can be passed multiple times to layer)")
	c.Flags().StringVar(&p.scratchDir, "scratch", "", "storage for root filesystem changes (required)")
	c.Flags().StringToStringVar(&p.mounts, "mount", nil, "mounts to set up")
	c.Flags().StringVar(&p.chdir, "workdir", "/", "working directory")
	return c
}

func (p *containerInitCmd) run(ctx context.Context, argv []string) error {
	if len(p.rootLayers) == 0 {
		return fmt.Errorf("must specify at least one --root layer")
	}
	if p.scratchDir == "" {
		return fmt.Errorf("must specify --scratch directory")
	}

	rootMount, err := createRoot(p.scratchDir, p.rootLayers)
	if err != nil {
		return err
	}
	if err := setupProc(rootMount); err != nil {
		return err
	}
	if err := setupDev(rootMount); err != nil {
		return err
	}
	for container, host := range p.mounts {
		dst := filepath.Join(rootMount, container)
		if err := os.MkdirAll(dst, 0777); err != nil {
			return fmt.Errorf("make mount %s: %w", container, err)
		}
		if err := bindMount(dst, host); err != nil {
			return fmt.Errorf("make mount %s: %w", container, err)
		}
	}
	if err := pivotRoot(rootMount); err != nil {
		return err
	}
	if err := os.Chdir(p.chdir); err != nil {
		return err
	}

	c := exec.CommandContext(ctx, argv[0], argv[1:]...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func createRoot(scratchDir string, layers []string) (string, error) {
	work := filepath.Join(scratchDir, "work")
	if err := os.Mkdir(work, 0700); err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("create root overlay: %w", err)
	}
	upperDir := filepath.Join(scratchDir, "upper")
	if err := os.Mkdir(upperDir, 0700); err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("create root overlay: %w", err)
	}
	rootMount := filepath.Join(scratchDir, "root")
	if err := os.Mkdir(rootMount, 0700); err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("create root overlay: %w", err)
	}
	if err := mountOverlay(rootMount, layers, upperDir, work); err != nil {
		return "", fmt.Errorf("create root overlay: %w", err)
	}
	return rootMount, nil
}

func setupProc(newRoot string) error {
	dst := filepath.Join(newRoot, "proc")
	if err := os.Mkdir(dst, 0777); err != nil && !os.IsExist(err) {
		return fmt.Errorf("mount /proc at %s: %w", dst, err)
	}
	if err := unix.Mount("proc", dst, "proc", 0, ""); err != nil {
		return fmt.Errorf("mount /proc at %s: %w", dst, err)
	}
	return nil
}

func setupDev(newRoot string) error {
	dst := filepath.Join(newRoot, "dev")
	if err := os.Mkdir(dst, 0777); err != nil && !os.IsExist(err) {
		return fmt.Errorf("mount /dev at %s: %w", dst, err)
	}
	devices := map[string]string{
		"null":    "/dev/null",
		"urandom": "/dev/urandom",
		// Intentionally linking /dev/random to /dev/urandom.
		// http://lists.randombit.net/pipermail/cryptography/2013-August/004983.html
		"random": "/dev/urandom",
		"zero":   "/dev/zero",
	}
	for name, dev := range devices {
		if err := makeDevice(dev, filepath.Join(dst, name)); err != nil {
			return fmt.Errorf("mount /dev at %s: %w", dst, err)
		}
	}
	return nil
}

func makeDevice(oldname, newname string) error {
	// Unprivileged containers can't create character devices, so we instead
	// bind mount the same device to the desired location.
	if err := unix.Mknod(newname, unix.S_IFREG|0o666, 0); err != nil && !os.IsExist(err) {
		return fmt.Errorf("copy device %s to %s: mknod: %w", oldname, newname, err)
	}
	if err := bindMount(newname, oldname); err != nil {
		return fmt.Errorf("copy device %s to %s: %w", oldname, newname, err)
	}
	return nil
}

func mountOverlay(dst string, lower []string, upper, work string) error {
	opts := new(strings.Builder)
	opts.WriteString("lowerdir=")
	for i, l := range lower {
		if i > 0 {
			opts.WriteByte(':')
		}
		opts.WriteString(mountEscaper.Replace(l))
	}
	opts.WriteString(",upperdir=")
	opts.WriteString(upper)
	opts.WriteString(",workdir=")
	opts.WriteString(work)
	if err := unix.Mount("overlay", dst, "overlay", 0, opts.String()); err != nil {
		return fmt.Errorf("mount overlay of %s onto %s at %s: %w", upper, lower, dst, err)
	}
	return nil
}

var mountEscaper = strings.NewReplacer(`\`, `\\`, `:`, `\:`)

func bindMount(dst, src string) error {
	if err := unix.Mount(src, dst, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
		return fmt.Errorf("bind %s onto %s: %w", src, dst, err)
	}
	return nil
}

func pivotRoot(newRoot string) error {
	// Create a bind mount of the new root.
	if err := bindMount(newRoot, newRoot); err != nil {
		return fmt.Errorf("mount %s as root: %w", newRoot, err)
	}

	// Temporary to swap?
	const tempName = ".temp"
	tempMount := filepath.Join(newRoot, tempName)
	if err := os.Mkdir(tempMount, 0700); err != nil {
		return fmt.Errorf("mount %s as root: %w", newRoot, err)
	}
	if err := unix.PivotRoot(newRoot, tempMount); err != nil {
		return fmt.Errorf("mount %s as root: %w", newRoot, err)
	}
	// pivot_root(2) recommends chdir("/") after calling pivot_root.
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("mount %s as root: %w", newRoot, err)
	}

	// Remove temporary mount.
	if err := unix.Unmount("/"+tempName, unix.MNT_DETACH); err != nil {
		return fmt.Errorf("mount %s as root: %w", newRoot, err)
	}
	if err := os.Remove("/" + tempName); err != nil {
		return fmt.Errorf("mount %s as root: %w", newRoot, err)
	}
	return nil
}
