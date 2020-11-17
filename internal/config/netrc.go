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

package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go4.org/xdgdir"
)

// DefaultNetrcFiles returns paths to search for netrc files in descending
// precedence order based on environment variables.
func DefaultNetrcFiles() []string {
	paths := xdgdir.Config.SearchPaths()
	netrcs := make([]string, 0, len(paths))
	for _, p := range paths {
		netrcs = append(netrcs, filepath.Join(p, dirName, "netrc"))
	}
	return netrcs
}

// CatFiles concatenates the given file contents in order. Files in the
// implicitFiles slice that don't exist will not cause CatFiles to return an
// error.
func CatFiles(implicitFiles, explicitFiles []string) ([]byte, error) {
	buf := new(bytes.Buffer)
	for _, fname := range implicitFiles {
		f, err := os.Open(fname)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(buf, f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", fname, err)
		}
	}
	for _, fname := range explicitFiles {
		f, err := os.Open(fname)
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(buf, f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", fname, err)
		}
	}
	return buf.Bytes(), nil
}
