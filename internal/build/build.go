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

// Package build provides the algorithm for building yb targets.
package build

import (
	"context"
	"fmt"
	slashpath "path"
	"strings"

	"github.com/google/shlex"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/buildpack"
	"github.com/yourbase/yb/internal/ybtrace"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
)

// Sys holds dependencies provided by the caller needed to run builds.
type Sys = buildpack.Sys

// Phase is a sequence of commands to run to accomplish an outcome.
//
// TODO(ch2285): This should be moved out of this package and into a separate
// loader package.
type Phase struct {
	TargetName string
	Commands   []string
	Root       string
}

// Execute runs the given phase. It assumes that the phase's dependencies are
// already available in the biome.
func Execute(ctx context.Context, sys Sys, target *Phase) (err error) {
	ctx, span := ybtrace.Start(ctx, "Build "+target.TargetName, trace.WithAttributes(
		label.String("target", target.TargetName),
	))
	defer func() {
		if err != nil {
			span.SetStatus(codes.Unknown, err.Error())
		}
		span.End()
	}()

	workDir := ""
	if target.Root != "" {
		if isSlashAbs(target.Root) {
			return fmt.Errorf("build %s: root %s is absolute", target.TargetName, target.Root)
		}
		workDir = joinSlashPath(sys.Biome, "", target.Root)
	}
	// Validate commands before running them.
	for _, cmdString := range target.Commands {
		if err := validateCommand(cmdString); err != nil {
			return fmt.Errorf("build %s: %w", target.TargetName, err)
		}
	}
	for _, cmdString := range target.Commands {
		newWorkDir, err := runCommand(ctx, sys, workDir, cmdString)
		if err != nil {
			return fmt.Errorf("build %s: %w", target.TargetName, err)
		}
		workDir = newWorkDir
	}
	return nil
}

func validateCommand(cmdString string) error {
	if dir, ok := parseChdir(cmdString); ok {
		if dir == "" {
			return fmt.Errorf("cd: empty directory")
		}
		if isSlashAbs(dir) {
			return fmt.Errorf("cd %s: absolute paths not supported", dir)
		}
		return nil
	}
	argv, err := shlex.Split(cmdString)
	if err != nil {
		return err
	}
	if len(argv) == 0 {
		return fmt.Errorf("empty build command")
	}
	return nil
}

func runCommand(ctx context.Context, sys Sys, dir string, cmdString string) (newDir string, err error) {
	ctx, span := ybtrace.Start(ctx, "Run "+cmdString, trace.WithAttributes(
		label.String("command", cmdString),
	))
	defer func() {
		if err != nil {
			span.SetStatus(codes.Unknown, err.Error())
		}
		span.End()
	}()

	if newDir, ok := parseChdir(cmdString); ok {
		// TODO(ch2195): What do we expect this to do in general?
		if isSlashAbs(newDir) {
			return dir, fmt.Errorf("run build command %q: cd: absolute path not allowed", cmdString)
		}
		return joinSlashPath(sys.Biome, dir, newDir), nil
	}
	argv, err := shlex.Split(cmdString)
	if err != nil {
		return dir, fmt.Errorf("run build command %q: %w", cmdString, err)
	}

	err = sys.Biome.Run(ctx, &biome.Invocation{
		Argv:   argv,
		Dir:    dir,
		Stdout: sys.Stdout,
		Stderr: sys.Stderr,
	})
	if err != nil {
		return dir, fmt.Errorf("run build command %q: %w", cmdString, err)
	}
	return dir, nil
}

func parseChdir(cmdString string) (dir string, ok bool) {
	const prefix = "cd "
	if !strings.HasPrefix(cmdString, prefix) {
		return "", false
	}
	return strings.TrimSpace(cmdString[len(prefix):]), true
}

func joinSlashPath(bio biome.Biome, dir, path string) string {
	parts := []string{dir}
	parts = append(parts, strings.Split(slashpath.Clean(path), "/")...)
	return bio.JoinPath(parts...)
}

// isSlashAbs reports whether the slash-separated path starts with a slash.
func isSlashAbs(path string) bool {
	return strings.HasPrefix(path, "/")
}
