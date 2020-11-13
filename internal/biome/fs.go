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
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// This file holds functions that can be derived from any implementation of the
// base biome interface, but may potentially have a more optimal implementation.

// CleanPath returns the shortest path name equivalent to path by purely
// lexical processing. It uses the same algorithm as path/filepath.Clean.
func CleanPath(bio Biome, path string) string {
	if path == "" {
		// JoinPath will return an empty string, which does not match Clean.
		return "."
	}
	return bio.JoinPath(path)
}

// AbsPath returns an absolute representation of path. If the path is not absolute
// it will be joined with the package directory to turn it into an absolute
// path. The absolute path name for a given file is not guaranteed to be unique.
// AbsPath calls Clean on the result.
func AbsPath(bio Biome, path string) string {
	if bio.IsAbsPath(path) {
		return CleanPath(bio, path)
	}
	return bio.JoinPath(bio.Dirs().Package, path)
}

type fileWriter interface {
	WriteFile(ctx context.Context, path string, src io.Reader) error
}

// WriteFile copies a file to the biome. Paths are resolved relative to
// the package directory.
//
// If the biome has a method
// `WriteFile(ctx context.Context, path string, src io.Reader) error`,
// that will be used. If it does not or the method returns ErrUnsupported,
// WriteFile will Run an appropriate fallback in the biome.
func WriteFile(ctx context.Context, bio Biome, path string, src io.Reader) error {
	if err := forwardWriteFile(ctx, bio, path, src); !errors.Is(err, ErrUnsupported) {
		return err
	}
	stderr := new(strings.Builder)
	err := bio.Run(ctx, &Invocation{
		Argv:   []string{"tee", path},
		Stdin:  src,
		Stderr: stderr,
	})
	if err != nil {
		if stderr.Len() == 0 {
			return fmt.Errorf("write file %s: %w", path, err)
		}
		return fmt.Errorf("write file %s: %s", path, strings.TrimSuffix(stderr.String(), "\n"))
	}
	return nil
}

func forwardWriteFile(ctx context.Context, bio Biome, path string, src io.Reader) error {
	writer, ok := bio.(fileWriter)
	if !ok {
		return fmt.Errorf("write file %s: %w", path, ErrUnsupported)
	}
	return writer.WriteFile(ctx, path, src)
}

type dirMaker interface {
	MkdirAll(ctx context.Context, path string) error
}

// MkdirAll creates a directory named path, along with any necessary parents,
// and returns nil, or else returns an error.
//
// If the biome has a method `MkdirAll(ctx context.Context, path string) error`,
// that will be used. If it does not or the method returns ErrUnsupported,
// MkdirAll will Run an appropriate fallback in the biome.
func MkdirAll(ctx context.Context, bio Biome, path string) error {
	if err := forwardMkdirAll(ctx, bio, path); !errors.Is(err, ErrUnsupported) {
		return err
	}
	stderr := new(strings.Builder)
	err := bio.Run(ctx, &Invocation{
		Argv:   []string{"mkdir", "-p", path},
		Stderr: stderr,
	})
	if err != nil {
		if stderr.Len() == 0 {
			return fmt.Errorf("mkdir -p %s: %w", path, err)
		}
		return fmt.Errorf("mkdir -p %s: %s", path, strings.TrimSuffix(stderr.String(), "\n"))
	}
	return nil
}

func forwardMkdirAll(ctx context.Context, bio Biome, path string) error {
	maker, ok := bio.(dirMaker)
	if !ok {
		return fmt.Errorf("mkdir -p %s: %w", path, ErrUnsupported)
	}
	return maker.MkdirAll(ctx, path)
}

type symlinkEvaler interface {
	EvalSymlinks(ctx context.Context, path string) (string, error)
}

// EvalSymlinks returns the path name after the evaluation of any symbolic links.
// Paths are resolved relative to the package directory. EvalSymlinks calls
// Clean on the result. If the path does not exist, EvalSymlinks returns an
// error.
//
// If the biome has a method `EvalSymlinks(ctx context.Context, path string) (string, error)`,
// that will be used. If it does not or the method returns ErrUnsupported,
// EvalSymlinks will Run an appropriate fallback in the biome.
func EvalSymlinks(ctx context.Context, bio Biome, path string) (string, error) {
	if resolved, err := forwardEvalSymlinks(ctx, bio, path); !errors.Is(err, ErrUnsupported) {
		return resolved, err
	}
	stdout := new(strings.Builder)
	stderr := new(strings.Builder)
	argv := []string{
		"python",
		"-c", `import os, sys; os.stat(sys.argv[1]); sys.stdout.write(os.path.realpath(sys.argv[1]))`,
		path,
	}
	if bio.Describe().OS == Linux {
		argv = []string{"readlink", "--canonicalize-existing", "--no-newline", path}
	}
	err := bio.Run(ctx, &Invocation{
		Argv:   argv,
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		if stderr.Len() == 0 {
			return "", fmt.Errorf("eval symlinks for %s: %w", path, err)
		}
		return "", fmt.Errorf("eval symlinks for %s: %s", path, strings.TrimSuffix(stderr.String(), "\n"))
	}
	return CleanPath(bio, stdout.String()), nil
}

func forwardEvalSymlinks(ctx context.Context, bio Biome, path string) (string, error) {
	evaler, ok := bio.(symlinkEvaler)
	if !ok {
		return "", fmt.Errorf("eval symlinks %s: %w", path, ErrUnsupported)
	}
	return evaler.EvalSymlinks(ctx, path)
}
