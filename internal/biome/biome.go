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

// Package biome provides an API for interacting with build environments.
package biome

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"zombiezen.com/go/log"
)

// ErrUnsupported indicates that a request operation cannot be performed,
// because it is unsupported.
//
// TODO(light): Replace with errors.ErrUnsupported when
// https://golang.org/issue/41198 is resolved.
var ErrUnsupported = errors.New("unsupported operation")

// A Biome is an environment that a target is built or run in.
// Implementations must be safe to use from multiple goroutines.
type Biome interface {
	// Describe returns information about the execution environment.
	// The caller must not modify the returned Descriptor.
	Describe() *Descriptor

	// Dirs returns paths to special directories.
	// The caller must not modify the returned Dirs.
	Dirs() *Dirs

	// Run runs a program specified by the given invocation and waits for
	// it to complete. Run must not modify any fields in the Invocation or
	// retain them after Run returns.
	Run(ctx context.Context, invoke *Invocation) error

	// The following methods are analogous to the ones in the
	// path/filepath package, but operate on the biome's filesystem paths.

	// JoinPath joins any number of path elements into a single path.
	// The result is cleaned as if by path/filepath.Clean, however, if the
	// argument list is empty or all its elements are empty, JoinPath
	// returns an empty string.
	JoinPath(elem ...string) string

	// IsAbsPath reports whether the path is absolute.
	IsAbsPath(path string) bool
}

// A Descriptor describes various facets of a biome.
type Descriptor struct {
	OS   string
	Arch string
}

// Equal reports whether two descriptors are equivalent.
func (desc *Descriptor) Equal(desc2 *Descriptor) bool {
	return *desc == *desc2
}

// Operating systems. Values are based off GOOS.
const (
	Linux   = "linux"
	MacOS   = "darwin"
	Windows = "windows"
)

// CPU Architectures. Values are based off GOARCH.
const (
	Intel64 = "amd64"
	Intel32 = "386"
	ARM64   = "arm64"
)

// Dirs holds paths to special directories in a Context.
type Dirs struct {
	// Package is the absolute path of the package directory.
	// Biomes guarantee this directory exists.
	Package string

	// Home is the absolute path of the build HOME directory.
	// Biomes guarantee this directory exists.
	Home string

	// Tools is the absolute path of a directory where helper tools can be
	// installed. It is not shared with other biomes. It may have to be
	// created.
	Tools string
}

// An Invocation holds the parameters for a single command.
type Invocation struct {
	// Argv is the argument list. Argv[0] is the name of the program to execute.
	Argv []string

	// Dir is the directory to execute the program in. Paths are resolved relative to
	// the package directory. If empty, then it will be executed in the package
	// directory. It is separated by the biome's path separator.
	Dir string

	// Env specifies additional environment variables to send to the program.
	// The biome may provide additional environment variables to the program, but
	// will not override the provided environment variables.
	Env Environment

	// Stdin specifies the program's standard input.
	// If Stdin is nil, the program reads from the null device.
	Stdin io.Reader

	// Interactive indicates whether the program will be surfaced to the user
	// interactively. This usually indicates that the program should be given
	// a pseudo-TTY as input.
	Interactive bool

	// Stdout and Stderr specify the program's standard output and error.
	// If either is nil, Run connects the corresponding file descriptor to the
	// null device.
	//
	// If Stdout and Stderr are the same writer, and have a type that can
	// be compared with ==, at most one goroutine at a time will call Write.
	Stdout io.Writer
	Stderr io.Writer
}

// Local is a biome that executes processes in a directory on the
// local machine.
type Local struct {
	// PackageDir is the absolute path to a directory containing the source files
	// of the package.
	PackageDir string

	// HomeDir is the absolute path to a directory that should be used as HOME.
	// This directory may or may not exist. It SHOULD NOT be the user's actual
	// home directory. It's meant for storing configuration and intermediate files
	// that any build tools need.
	HomeDir string
}

// Describe returns the values of GOOS/GOARCH.
func (l Local) Describe() *Descriptor {
	return &Descriptor{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}
}

// Dirs returns special directories.
func (l Local) Dirs() *Dirs {
	if !filepath.IsAbs(l.PackageDir) {
		panic("Local.PackageDir is not absolute")
	}
	if !filepath.IsAbs(l.HomeDir) {
		panic("Local.HomeDir is not absolute")
	}
	return &Dirs{
		Package: l.PackageDir,
		Home:    l.HomeDir,
		Tools:   filepath.Join(l.HomeDir, ".cache", "yb", "tools"),
	}
}

// Run runs a subprocess and waits for it to exit.
func (l Local) Run(ctx context.Context, invoke *Invocation) error {
	if len(invoke.Argv) == 0 {
		return fmt.Errorf("local run: argv empty")
	}
	log.Debugf(ctx, "Run: %s", strings.Join(invoke.Argv, " "))
	log.Debugf(ctx, "Environment:\n%v", invoke.Env)
	dir := invoke.Dir
	if !filepath.IsAbs(invoke.Dir) {
		dir = filepath.Join(l.PackageDir, invoke.Dir)
	}
	program, err := l.lookPath(invoke.Env, dir, invoke.Argv[0])
	if err != nil {
		return fmt.Errorf("local run: %w", err)
	}
	log.Debugf(ctx, "Program = %s", program)
	c := exec.CommandContext(ctx, program, invoke.Argv[1:]...)
	c.Env = []string{
		"HOME=" + l.HomeDir,
		"LOGNAME=" + os.Getenv("LOGNAME"),
		"USER=" + os.Getenv("USER"),
	}
	if v, ok := os.LookupEnv("NO_COLOR"); ok {
		c.Env = append(c.Env, "NO_COLOR="+v)
	}
	c.Env = appendStandardEnv(c.Env, runtime.GOOS)
	c.Env = invoke.Env.appendTo(c.Env, os.Getenv("PATH"), filepath.ListSeparator)
	c.Dir = dir
	c.Stdin = invoke.Stdin
	c.Stdout = invoke.Stdout
	c.Stderr = invoke.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("local run: %w", err)
	}
	return nil
}

func appendStandardEnv(env []string, biomeOS string) []string {
	env = append(env, "TZ=UTC0")
	if biomeOS == MacOS {
		env = append(env, "LANG=C", "LC_CTYPE=UTF-8")
	} else {
		env = append(env, "LANG=C.UTF-8", "LC_ALL=C.UTF-8")
	}
	return env
}

func (l Local) lookPath(env Environment, dir string, program string) (string, error) {
	abs := func(path string) string {
		if filepath.IsAbs(path) {
			return filepath.Clean(path)
		}
		return filepath.Join(dir, path)
	}
	if strings.ContainsRune(program, filepath.Separator) {
		return exec.LookPath(abs(program))
	}
	envPATH := env.computePATH(os.Getenv("PATH"), filepath.ListSeparator)
	envPATH = envPATH[len("PATH="):]
	for _, p := range filepath.SplitList(envPATH) {
		if found, err := exec.LookPath(abs(filepath.Join(p, program))); err == nil {
			return found, nil
		}
	}
	return "", &exec.Error{Name: program, Err: exec.ErrNotFound}
}

// JoinPath calls filepath.Join.
func (l Local) JoinPath(elem ...string) string {
	return filepath.Join(elem...)
}

// IsAbsPath calls filepath.IsAbs.
func (l Local) IsAbsPath(path string) bool {
	return filepath.IsAbs(path)
}

// WriteFile writes the data from src to the gven path with the mode 0666.
func (l Local) WriteFile(ctx context.Context, path string, src io.Reader) error {
	f, err := os.Create(AbsPath(l, path))
	if err != nil {
		return err
	}
	_, writeErr := io.Copy(f, src)
	closeErr := f.Close()
	if writeErr != nil {
		return fmt.Errorf("write file %s: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("write file %s: %w", path, closeErr)
	}
	return nil
}

// MkdirAll calls os.MkdirAll(path, 0777).
func (l Local) MkdirAll(ctx context.Context, path string) error {
	return os.MkdirAll(AbsPath(l, path), 0777)
}

// EvalSymlinks calls filepath.EvalSymlinks.
func (l Local) EvalSymlinks(ctx context.Context, path string) (string, error) {
	return filepath.EvalSymlinks(AbsPath(l, path))
}

// Close does nothing and returns nil.
func (l Local) Close() error {
	return nil
}

// ExecPrefix intercepts calls to Run and prepends elements to the Argv slice.
// This can be used to invoke tools with a wrapping command like `time` or `sudo`.
type ExecPrefix struct {
	Biome
	PrependArgv []string
}

// Run calls ep.Biome.Run with an invocation whose Argv is the result of
// appending invoke.Argv to ep.PrependArgv.Argv.
func (ep ExecPrefix) Run(ctx context.Context, invoke *Invocation) error {
	if len(ep.PrependArgv) == 0 {
		return ep.Biome.Run(ctx, invoke)
	}
	invoke2 := new(Invocation)
	*invoke2 = *invoke
	invoke2.Argv = make([]string, 0, len(ep.PrependArgv)+len(invoke.Argv))
	invoke2.Argv = append(invoke2.Argv, ep.PrependArgv...)
	invoke2.Argv = append(invoke2.Argv, invoke.Argv...)
	return ep.Biome.Run(ctx, invoke2)
}

// WriteFile calls ep.Context.WriteFile or returns ErrUnsupported if not present.
func (ep ExecPrefix) WriteFile(ctx context.Context, path string, src io.Reader) error {
	return forwardWriteFile(ctx, ep.Biome, path, src)
}

// MkdirAll calls ep.Context.MkdirAll or returns ErrUnsupported if not present.
func (ep ExecPrefix) MkdirAll(ctx context.Context, path string) error {
	return forwardMkdirAll(ctx, ep.Biome, path)
}

// EvalSymlinks calls ep.Context.EvalSymlinks or returns ErrUnsupported if not present.
func (ep ExecPrefix) EvalSymlinks(ctx context.Context, path string) (string, error) {
	return forwardEvalSymlinks(ctx, ep.Biome, path)
}

// Close calls ep.Biome.Close if such a method exists or returns nil if not present.
func (ep ExecPrefix) Close() error {
	if c, ok := ep.Biome.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
