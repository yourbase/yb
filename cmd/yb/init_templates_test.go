// Copyright 2020 YourBase Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//		 https://www.apache.org/licenses/LICENSE-2.0
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
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/yourbase/yb"
)

func TestPackageConfigTemplates(t *testing.T) {
	for name, data := range packageConfigTemplates {
		if name == "" {
			name = "Generic"
		}
		t.Run(name, func(t *testing.T) {
			dst := filepath.Join(t.TempDir(), yb.PackageConfigFilename)
			if err := ioutil.WriteFile(dst, []byte(data), 0o666); err != nil {
				t.Fatal(err)
			}
			if _, err := yb.LoadPackage(dst); err != nil {
				t.Errorf("Load file: %v", err)
			}
		})
	}
}
