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
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"zombiezen.com/go/log/testlog"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  string
	}{
		{
			name:  "EmptyDir",
			files: []string{},
			want:  langGenericFlagValue,
		},
		{
			name: "Go/Mod",
			files: []string{
				"go.mod",
			},
			want: langGoFlagValue,
		},
		{
			name: "Go/JustSource",
			files: []string{
				"xyzzy.go",
			},
			want: langGoFlagValue,
		},
		{
			name: "Python/PipRequirements",
			files: []string{
				"requirements.txt",
			},
			want: langPythonFlagValue,
		},
		{
			name: "Python/Setuptools",
			files: []string{
				"setup.py",
			},
			want: langPythonFlagValue,
		},
		{
			name: "Ruby",
			files: []string{
				"Gemfile",
			},
			want: langRubyFlagValue,
		},
		{
			name: "Makefile",
			files: []string{
				"Makefile",
			},
			want: langGenericFlagValue,
		},
		{
			name: "MultipleLanguages",
			files: []string{
				"go.mod",
				"setup.py",
			},
			want: langGenericFlagValue,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := testlog.WithTB(context.Background(), t)
			dir := t.TempDir()
			for _, fname := range test.files {
				dst := filepath.Join(dir, filepath.FromSlash(fname))
				if err := os.MkdirAll(filepath.Dir(dst), 0o777); err != nil {
					t.Fatal(err)
				}
				if err := ioutil.WriteFile(dst, nil, 0o666); err != nil {
					t.Fatal(err)
				}
			}
			got, err := detectLanguage(ctx, dir)
			if got != test.want || err != nil {
				t.Errorf("detectLanguage(%q) = %q, %v; want %q, <nil>", dir, got, err, test.want)
			}
		})
	}
}
