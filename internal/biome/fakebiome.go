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
	"fmt"
	slashpath "path"
	"strings"
)

// Fake is a biome that operates in-memory. It uses POSIX-style paths, but
// permits any character to be used as the separator.
type Fake struct {
	// Separator is the path separator character. If NUL, then slash '/' is used.
	Separator rune

	// Descriptor is the descriptor that will be returned by Describe.
	Descriptor Descriptor

	// DirsResult is what will be returned by Dirs.
	DirsResult Dirs

	// RunFunc is called to handle the Run method.
	RunFunc func(context.Context, *Invocation) error
}

func (f *Fake) sep() rune {
	if f.Separator == 0 {
		return '/'
	}
	return f.Separator
}

// Describe returns f.Descriptor.
func (f *Fake) Describe() *Descriptor {
	return &f.Descriptor
}

// Dirs returns f.DirsResult.
func (f *Fake) Dirs() *Dirs {
	return &f.DirsResult
}

// Run calls f.RunFunc. It returns an error if f.RunFunc is nil.
func (f *Fake) Run(ctx context.Context, invoke *Invocation) error {
	if f.RunFunc == nil {
		return fmt.Errorf("fake run: RunFunc not set")
	}
	return f.RunFunc(ctx, invoke)
}

// JoinPath joins any number of path elements into a single path.
func (f *Fake) JoinPath(elem ...string) string {
	sb := new(strings.Builder)
	for _, e := range elem {
		if e == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteRune(f.sep())
		}
		sb.WriteString(e)
	}
	if sb.Len() == 0 {
		return ""
	}
	return f.cleanPath(sb.String())
}

// cleanPath returns the shortest path name equivalent to path by purely
// lexical processing.
func (f *Fake) cleanPath(path string) string {
	if f.sep() == '/' {
		return slashpath.Clean(path)
	}
	s := strings.NewReplacer("/", "\x00", string(f.sep()), "/").Replace(path)
	s = slashpath.Clean(s)
	return strings.NewReplacer("/", string(f.sep()), "\x00", "/").Replace(s)
}

// IsAbsPath reports whether the path is absolute.
func (f *Fake) IsAbsPath(path string) bool {
	return strings.HasPrefix(path, string(f.sep()))
}

// Close does nothing and returns nil.
func (f *Fake) Close() error {
	return nil
}
