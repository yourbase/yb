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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"zombiezen.com/go/log"
)

// A Biome is an environment that a target is built or run in.
// Implementations must be safe to use from multiple goroutines.
type Biome interface {
	// Describe returns information about the execution environment.
	// The caller must not modify the returned Descriptor.
	Describe() *Descriptor

	// Run runs a program specified by the given invocation and waits for
	// it to complete. Run must not modify any fields in the Invocation or
	// retain them after Run returns.
	Run(ctx context.Context, invoke *Invocation) error

	// The following methods are analogous to the ones in the
	// path/filepath package, but operate on the biome's filesystem paths.

	// JoinPath joins any number of path elements into a single path.
	JoinPath(elem ...string) string

	// CleanPath returns the shortest path name equivalent to path by purely
	// lexical processing.
	CleanPath(path string) string

	// IsAbsPath reports whether the path is absolute.
	IsAbsPath(path string) bool

	// PathFromSlash returns the result of replacing each slash ('/')
	// character in path with a separator character.
	PathFromSlash(path string) string
}

// A Descriptor describes various facets of a biome.
type Descriptor struct {
	OS   string
	Arch string
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
	// PackageDir is a directory containing the source files of the package.
	PackageDir string
}

// Describe returns the values of GOOS/GOARCH.
func (l Local) Describe() *Descriptor {
	return &Descriptor{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}
}

// Run runs a subprocess and waits for it to exit.
func (l Local) Run(ctx context.Context, invoke *Invocation) error {
	if len(invoke.Argv) == 0 {
		return fmt.Errorf("local run: argv empty")
	}
	log.Infof(ctx, "Run: %s", strings.Join(invoke.Argv, " "))
	log.Debugf(ctx, "Environment:\n%v", invoke.Env)
	program, err := lookPath(invoke.Env, invoke.Argv[0])
	if err != nil {
		return fmt.Errorf("local run: %w", err)
	}
	log.Debugf(ctx, "Program = %s", program)
	c := exec.CommandContext(ctx, program, invoke.Argv[1:]...)
	// TODO(ch2744): This appends to os.Environ because the buildpacks
	// depend on being able to set environment variables.
	if !invoke.Env.IsEmpty() {
		c.Env = invoke.Env.appendTo(os.Environ(), os.Getenv("PATH"), filepath.ListSeparator)
	}
	if filepath.IsAbs(invoke.Dir) {
		c.Dir = invoke.Dir
	} else {
		c.Dir = filepath.Join(l.PackageDir, invoke.Dir)
	}
	c.Stdin = invoke.Stdin
	c.Stdout = invoke.Stdout
	c.Stderr = invoke.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("local run: %w", err)
	}
	return nil
}

func lookPath(env Environment, program string) (string, error) {
	if strings.Contains(program, string(filepath.Separator)) {
		return exec.LookPath(program)
	}
	envPATH := env.computePATH(os.Getenv("PATH"), filepath.ListSeparator)
	envPATH = envPATH[len("PATH="):]
	for _, p := range filepath.SplitList(envPATH) {
		if found, err := exec.LookPath(filepath.Join(p, program)); err == nil {
			return found, nil
		}
	}
	return "", &exec.Error{Name: program, Err: exec.ErrNotFound}
}

// JoinPath calls filepath.Join.
func (l Local) JoinPath(elem ...string) string {
	return filepath.Join(elem...)
}

// CleanPath calls filepath.Clean.
func (l Local) CleanPath(path string) string {
	return filepath.Join(path)
}

// IsAbsPath calls filepath.IsAbs.
func (l Local) IsAbsPath(path string) bool {
	return filepath.IsAbs(path)
}

// PathFromSlash calls filepath.FromSlash.
func (l Local) PathFromSlash(path string) string {
	return filepath.FromSlash(path)
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