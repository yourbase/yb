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

package buildpack

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/types"
	"zombiezen.com/go/log/testlog"
)

func TestAnaconda(t *testing.T) {
	tests := []struct {
		name string
		spec types.BuildpackSpec
	}{
		{
			name: "Python2",
			spec: "anaconda2:4.8.3",
		},
		{
			name: "Python3",
			spec: "anaconda3:4.8.3",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := testlog.WithTB(context.Background(), t)
			anacondaCtx, _ := testInstall(ctx, t, test.spec)
			infoOutput := new(strings.Builder)
			err := anacondaCtx.Run(ctx, &biome.Invocation{
				Argv:   []string{"conda", "info"},
				Stdout: infoOutput,
				Stderr: infoOutput,
			})
			t.Logf("conda info:\n%s", infoOutput)
			if err != nil {
				t.Errorf("conda info: %v", err)
			}
		})
	}
}

func TestAnacondaDownloadURL(t *testing.T) {
	linux := &biome.Descriptor{
		OS:   biome.Linux,
		Arch: biome.Intel64,
	}
	tests := []struct {
		version   string
		pyMajor   int
		pyMinor   int
		desc      *biome.Descriptor
		want      string
		wantError bool
	}{
		{
			version: "4.8.3",
			pyMajor: 3,
			pyMinor: 7,
			desc:    linux,
			want:    "https://repo.continuum.io/miniconda/Miniconda3-py37_4.8.3-Linux-x86_64.sh",
		},
		{
			version: "4.8.3",
			pyMajor: 3,
			pyMinor: 8,
			desc:    linux,
			want:    "https://repo.continuum.io/miniconda/Miniconda3-py38_4.8.3-Linux-x86_64.sh",
		},
		{
			version: "4.8.3",
			pyMajor: 2,
			pyMinor: 7,
			desc:    linux,
			want:    "https://repo.continuum.io/miniconda/Miniconda2-py27_4.8.3-Linux-x86_64.sh",
		},
		{
			version: "4.7.10",
			pyMajor: 3,
			pyMinor: 7,
			desc:    linux,
			want:    "https://repo.continuum.io/miniconda/Miniconda3-4.7.10-Linux-x86_64.sh",
		},
		{
			version: "4.7.10",
			pyMajor: 3,
			pyMinor: 8,
			desc:    linux,
			want:    "https://repo.continuum.io/miniconda/Miniconda3-4.7.10-Linux-x86_64.sh",
		},
		{
			version: "4.7.10",
			pyMajor: 2,
			pyMinor: 7,
			desc:    linux,
			want:    "https://repo.continuum.io/miniconda/Miniconda2-4.7.10-Linux-x86_64.sh",
		},
	}
	for _, test := range tests {
		got, err := anacondaDownloadURL(test.version, test.pyMajor, test.pyMinor, test.desc)
		if got != test.want || (err != nil) != test.wantError {
			errString := "<nil>"
			if test.wantError {
				errString = "<error>"
			}
			t.Errorf("anacondaDownloadURL(%q, %d, %d) = %q, %v; want %q, %s", test.version, test.pyMajor, test.pyMinor, got, err, test.want, errString)
		}
	}

	t.Run("Existence", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping due to -short")
		}
		for _, test := range tests {
			if !test.wantError {
				verifyURLExists(t, http.MethodHead, test.want)
			}
		}
	})
}
