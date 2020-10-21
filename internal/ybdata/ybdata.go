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

// Package ybdata locates directories the user has designated or conventionally
// uses for storing different types of data.
package ybdata

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yourbase/yb/internal/biome"
	"go4.org/xdgdir"
)

// Dirs is the result of locating directories.
type Dirs struct {
	cache      string
	workspaces string
}

// DirsFromEnv finds data directories based on environment variables.
func DirsFromEnv() (*Dirs, error) {
	cache := os.Getenv("YB_CACHE_DIR")
	if cache == "" {
		// TODO(light): This should use LocalAppData on Windows.
		rootCache := xdgdir.Cache.Path()
		if rootCache == "" {
			return nil, fmt.Errorf("neither YB_CACHE_DIR nor %v set", xdgdir.Cache)
		}
		cache = filepath.Join(rootCache, "yb")
	}
	workspaces := os.Getenv("YB_WORKSPACES_ROOT")
	if workspaces == "" {
		workspaces = filepath.Join(cache, "workspaces")
	}
	return &Dirs{
		cache:      cache,
		workspaces: workspaces,
	}, nil
}

// NewDirs returns a set of directories that is enclosed within a root directory.
// This is useful for isolating test data.
func NewDirs(root string) *Dirs {
	return &Dirs{
		cache:      root,
		workspaces: filepath.Join(root, "workspaces"),
	}
}

// Downloads returns the top-level directory to store downloaded files.
// This directory may not exist yet.
func (dirs *Dirs) Downloads() string {
	return filepath.Join(dirs.cache, "downloads")
}

// BuildHome finds or creates a directory to store cached data for a target.
//
// TODO(ch2755): This should get moved to a directory physically in the package
// directory.
func (dirs *Dirs) BuildHome(packageDir, target string, desc *biome.Descriptor) (string, error) {
	h := sha256.Sum256([]byte(packageDir))
	workspaceHash := hex.EncodeToString(h[:hex.DecodedLen(12)])
	path := filepath.Join(dirs.workspaces, workspaceHash, target, desc.OS, desc.Arch)
	if err := os.MkdirAll(path, 0777); err != nil {
		return "", fmt.Errorf("create build home: %w", err)
	}
	return path, nil
}
