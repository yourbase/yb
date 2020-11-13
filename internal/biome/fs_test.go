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
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zombiezen.com/go/log/testlog"
)

func TestWriteFile(t *testing.T) {
	junkHome := t.TempDir()
	tests := []struct {
		name     string
		newBiome func(dir string) Biome
	}{
		{
			name: "Local",
			newBiome: func(dir string) Biome {
				return Local{
					PackageDir: dir,
					HomeDir:    junkHome,
				}
			},
		},
		{
			name: "Fallback",
			newBiome: func(dir string) Biome {
				return forceFallback{Local{
					PackageDir: dir,
					HomeDir:    junkHome,
				}}
			},
		},
		{
			name: "Unsupported",
			newBiome: func(dir string) Biome {
				return unsupported{Local{
					PackageDir: dir,
					HomeDir:    junkHome,
				}}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := testlog.WithTB(context.Background(), t)
			dir := t.TempDir()
			bio := test.newBiome(dir)

			const fname = "foo.txt"
			const want = "Hello, World!\n"
			err := WriteFile(ctx, bio, fname, strings.NewReader(want))
			if err != nil {
				t.Error("WriteFile:", err)
			}

			got, err := ioutil.ReadFile(filepath.Join(dir, fname))
			if err != nil {
				t.Fatal("ReadFile:", err)
			}
			if string(got) != want {
				t.Errorf("%s content = %q; want %q", fname, got, want)
			}
		})
	}
}

func TestMkdirAll(t *testing.T) {
	junkHome := t.TempDir()
	tests := []struct {
		name     string
		newBiome func(dir string) Biome
	}{
		{
			name: "Local",
			newBiome: func(dir string) Biome {
				return Local{
					PackageDir: dir,
					HomeDir:    junkHome,
				}
			},
		},
		{
			name: "Fallback",
			newBiome: func(dir string) Biome {
				return forceFallback{Local{
					PackageDir: dir,
					HomeDir:    junkHome,
				}}
			},
		},
		{
			name: "Unsupported",
			newBiome: func(dir string) Biome {
				return unsupported{Local{
					PackageDir: dir,
					HomeDir:    junkHome,
				}}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := testlog.WithTB(context.Background(), t)
			dir := t.TempDir()
			bio := test.newBiome(dir)

			createdDir := filepath.Join("foo", "bar")
			err := MkdirAll(ctx, bio, createdDir)
			if err != nil {
				t.Error("MkdirAll:", err)
			}

			got, err := os.Stat(filepath.Join(dir, createdDir))
			if err != nil {
				t.Fatal(err)
			}
			if !got.IsDir() {
				t.Errorf("%s is not a directory", createdDir)
			}
		})
	}
}

func TestEvalSymlinks(t *testing.T) {
	// Set up directory.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	const fname = "foo.txt"
	const missingFile = "bork.txt"
	if err := ioutil.WriteFile(filepath.Join(dir, fname), nil, 0666); err != nil {
		t.Fatal(err)
	}
	// Don't fail test if we can't create the symlink. We're probably on Windows.
	// We'll bubble up the failure by skipping the Symlink tests below.
	const linkName = "mylink.txt"
	symlinkErr := os.Symlink(fname, filepath.Join(dir, linkName))

	tests := []struct {
		name string
		bio  Biome
	}{
		{
			name: "Local",
			bio: Local{
				PackageDir: dir,
				HomeDir:    home,
			},
		},
		{
			name: "Fallback",
			bio: forceFallback{Local{
				PackageDir: dir,
				HomeDir:    home,
			}},
		},
		{
			name: "Unsupported",
			bio: unsupported{Local{
				PackageDir: dir,
				HomeDir:    home,
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Run("Regular", func(t *testing.T) {
				ctx := testlog.WithTB(context.Background(), t)
				got, err := EvalSymlinks(ctx, test.bio, fname)
				if err != nil {
					t.Fatal("EvalSymlinks:", err)
				}
				if gotAbs, want := AbsPath(test.bio, got), filepath.Join(dir, fname); gotAbs != want {
					t.Errorf("EvalSymlinks(ctx, bio, %q) = %q (Abs=%q), <nil>; want Abs=%q, <nil>", fname, got, gotAbs, want)
				}
				if got != CleanPath(test.bio, got) {
					t.Errorf("Path %q is not clean", got)
				}
			})
			t.Run("DoesNotExist", func(t *testing.T) {
				ctx := testlog.WithTB(context.Background(), t)
				got, err := EvalSymlinks(ctx, test.bio, missingFile)
				if err == nil {
					t.Errorf("EvalSymlinks(ctx, bio, %q) = %q, <nil>; want _, <error>", missingFile, got)
				}
			})
			t.Run("Symlink", func(t *testing.T) {
				if symlinkErr != nil {
					t.Skip("Could not symlink:", symlinkErr)
				}
				ctx := testlog.WithTB(context.Background(), t)
				got, err := EvalSymlinks(ctx, test.bio, linkName)
				if err != nil {
					t.Fatal("EvalSymlinks:", err)
				}
				if gotAbs, want := AbsPath(test.bio, got), filepath.Join(dir, fname); gotAbs != want {
					t.Errorf("EvalSymlinks(ctx, bio, %q) = %q (Abs=%q), <nil>; want Abs=%q, <nil>", linkName, got, gotAbs, want)
				}
				if got != CleanPath(test.bio, got) {
					t.Errorf("Path %q is not clean", got)
				}
			})
		})
	}
}

// forceFallback delegates the minimal biome method set to another biome.
// This forces functions that test for extra methods on a biome to fall back
// to the default implementation.
type forceFallback struct {
	Biome
}

// unsupported delegates the minimal biome method set to another biome.
// Any optional interfaces are implemented but return ErrUnsupported.
type unsupported struct {
	Biome
}

func (unsupported) WriteFile(ctx context.Context, path string, src io.Reader) error {
	return fmt.Errorf("write file %s: %w", path, ErrUnsupported)
}

func (unsupported) MkdirAll(ctx context.Context, path string) error {
	return fmt.Errorf("mkdir -p %s: %w", path, ErrUnsupported)
}

func (unsupported) EvalSymlinks(ctx context.Context, path string) (string, error) {
	return "", fmt.Errorf("eval symlinks %s: %w", path, ErrUnsupported)
}

var _ interface {
	fileWriter
	dirMaker
	symlinkEvaler
} = unsupported{}
