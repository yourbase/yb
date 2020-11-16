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
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCatFiles(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "file1.txt")
	const content1 = "Hello\n"
	if err := ioutil.WriteFile(file1, []byte(content1), 0666); err != nil {
		t.Fatal(err)
	}
	file2 := filepath.Join(dir, "file2.txt")
	const content2 = "omglolwut\n"
	if err := ioutil.WriteFile(file2, []byte(content2), 0666); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		implicitFiles []string
		explicitFiles []string
		want          string
		wantErr       bool
	}{
		{
			name: "Empty",
			want: "",
		},
		{
			name:          "SingleImplicit",
			implicitFiles: []string{file1},
			want:          content1,
		},
		{
			name:          "SingleExplicit",
			explicitFiles: []string{file1},
			want:          content1,
		},
		{
			name:          "CombinedImplicit",
			implicitFiles: []string{file1, file2},
			want:          content1 + content2,
		},
		{
			name:          "CombinedExplicit",
			explicitFiles: []string{file1, file2},
			want:          content1 + content2,
		},
		{
			name:          "ImplicitAndExplicit",
			implicitFiles: []string{file1},
			explicitFiles: []string{file2},
			want:          content1 + content2,
		},
		{
			name:          "ImplicitDoesNotExist",
			implicitFiles: []string{filepath.Join(dir, "bork.txt")},
			explicitFiles: []string{file2},
			want:          content2,
		},
		{
			name:          "ExplicitDoesNotExist",
			implicitFiles: []string{file1},
			explicitFiles: []string{filepath.Join(dir, "bork.txt")},
			wantErr:       true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := CatFiles(test.implicitFiles, test.explicitFiles)
			if err != nil {
				t.Log("CatFiles:", err)
				if !test.wantErr {
					t.Fail()
				}
				return
			}
			if test.wantErr {
				t.Fatalf("CatFiles(...) = %q, <nil>; want _, <error>", got)
			}
			if diff := cmp.Diff(test.want, string(got)); diff != "" {
				t.Errorf("CatFiles(...) (-want +got):\n%s", diff)
			}
		})
	}
}
