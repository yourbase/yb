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
	"io"
	"sort"
	"strings"
)

// Environment holds environment variables. The zero value is an empty
// environment.
type Environment struct {
	// Vars is a mapping of variables.
	Vars map[string]string
	// PrependPath is a list of paths to prepend to PATH.
	PrependPath []string
	// AppendPath is a list of paths to append to PATH.
	AppendPath []string
}

// IsEmpty reports whether env contains no variables.
func (env Environment) IsEmpty() bool {
	return len(env.Vars) == 0 && len(env.PrependPath) == 0 && len(env.AppendPath) == 0
}

// Merge returns a new environment that merges env2 into env.
func (env Environment) Merge(env2 Environment) Environment {
	env3 := Environment{
		Vars:        make(map[string]string),
		PrependPath: append(env2.PrependPath[:len(env2.PrependPath):len(env2.PrependPath)], env.PrependPath...),
		AppendPath:  append(env.AppendPath[:len(env.AppendPath):len(env.AppendPath)], env2.AppendPath...),
	}
	for k, v := range env.Vars {
		env3.Vars[k] = v
	}
	for k, v := range env2.Vars {
		env3.Vars[k] = v
	}
	return env3
}

const pathVar = "PATH"

// appendTo appends a sorted list of variables in the form "key=value" to the
// string slice. If PATH is not present in env.Vars, then defaultPath is used.
func (env Environment) appendTo(dst []string, defaultPath string, pathListSep rune) []string {
	keys := make([]string, 0, len(env.Vars)+1)
	hasPATH := defaultPath != "" || env.hasPATH()
	if hasPATH {
		keys = append(keys, pathVar)
	}
	for k := range env.Vars {
		if k == pathVar {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if k == pathVar {
			dst = append(dst, env.computePATH(defaultPath, pathListSep))
		} else {
			dst = append(dst, k+"="+env.Vars[k])
		}
	}
	return dst
}

func (env Environment) hasPATH() bool {
	if len(env.PrependPath)+len(env.AppendPath) > 0 {
		return true
	}
	_, exists := env.Vars[pathVar]
	return exists
}

func (env Environment) computePATH(defaultPath string, pathListSep rune) string {
	sb := new(strings.Builder)
	sb.WriteString(pathVar)
	sb.WriteString("=")
	init := sb.Len()
	for _, v := range env.PrependPath {
		if sb.Len() > init {
			sb.WriteRune(pathListSep)
		}
		sb.WriteString(v)
	}
	path := env.Vars[pathVar]
	if path == "" {
		path = defaultPath
	}
	if path != "" {
		if sb.Len() > init {
			sb.WriteRune(pathListSep)
		}
		sb.WriteString(path)
	}
	for _, v := range env.AppendPath {
		if sb.Len() > init {
			sb.WriteRune(pathListSep)
		}
		sb.WriteString(v)
	}
	return sb.String()
}

// String formats the environment variables one per line sorted by variable name.
// This output is for debugging purposes only and should not be depended upon.
func (env Environment) String() string {
	parts := env.appendTo(nil, "", ':')
	return strings.Join(parts, "\n")
}

// EnvBiome wraps a biome to add a base environment to any run commands.
type EnvBiome struct {
	Biome
	Env Environment
}

// Run runs a command with eb.Env as a base environment with invoke.Env
// merged in.
func (eb EnvBiome) Run(ctx context.Context, invoke *Invocation) error {
	if eb.Env.IsEmpty() {
		return eb.Biome.Run(ctx, invoke)
	}
	invoke2 := new(Invocation)
	*invoke2 = *invoke
	invoke2.Env = eb.Env.Merge(invoke.Env)
	return eb.Biome.Run(ctx, invoke2)
}

// WriteFile calls eb.Context.WriteFile or returns ErrUnsupported if not present.
func (eb EnvBiome) WriteFile(ctx context.Context, path string, src io.Reader) error {
	return forwardWriteFile(ctx, eb.Biome, path, src)
}

// MkdirAll calls eb.Context.MkdirAll or returns ErrUnsupported if not present.
func (eb EnvBiome) MkdirAll(ctx context.Context, path string) error {
	return forwardMkdirAll(ctx, eb.Biome, path)
}

// EvalSymlinks calls eb.Context.EvalSymlinks or returns ErrUnsupported if not present.
func (eb EnvBiome) EvalSymlinks(ctx context.Context, path string) (string, error) {
	return forwardEvalSymlinks(ctx, eb.Biome, path)
}

// Close calls eb.Biome.Close if such a method exists or returns nil if not present.
func (eb EnvBiome) Close() error {
	if c, ok := eb.Biome.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
