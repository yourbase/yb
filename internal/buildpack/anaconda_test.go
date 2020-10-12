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
	"net/http"
	"testing"

	"github.com/yourbase/commons/http/headers"
)

func TestAnacondaDownloadURL(t *testing.T) {
	osArchGolden := anacondaOSMap[OS()] + "-" + anacondaArchMap[Arch()]
	tests := []struct {
		version   string
		pyMajor   int
		pyMinor   int
		want      string
		wantError bool
	}{
		{
			version: "4.8.3",
			pyMajor: 3,
			pyMinor: 7,
			want:    "https://repo.continuum.io/miniconda/Miniconda3-py37_4.8.3-" + osArchGolden + ".sh",
		},
		{
			version: "4.8.3",
			pyMajor: 3,
			pyMinor: 8,
			want:    "https://repo.continuum.io/miniconda/Miniconda3-py38_4.8.3-" + osArchGolden + ".sh",
		},
		{
			version: "4.8.3",
			pyMajor: 2,
			pyMinor: 7,
			want:    "https://repo.continuum.io/miniconda/Miniconda2-py27_4.8.3-" + osArchGolden + ".sh",
		},
		{
			version: "4.7.10",
			pyMajor: 3,
			pyMinor: 7,
			want:    "https://repo.continuum.io/miniconda/Miniconda3-4.7.10-" + osArchGolden + ".sh",
		},
		{
			version: "4.7.10",
			pyMajor: 3,
			pyMinor: 8,
			want:    "https://repo.continuum.io/miniconda/Miniconda3-4.7.10-" + osArchGolden + ".sh",
		},
		{
			version: "4.7.10",
			pyMajor: 2,
			pyMinor: 7,
			want:    "https://repo.continuum.io/miniconda/Miniconda2-4.7.10-" + osArchGolden + ".sh",
		},
	}
	for _, test := range tests {
		got, err := anacondaDownloadURL(test.version, test.pyMajor, test.pyMinor)
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
			if test.wantError {
				continue
			}
			resp, err := http.Head(test.want)
			if err != nil {
				t.Errorf("FAIL %s: %v", test.want, err)
				continue
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("FAIL %s: HTTP %s", test.want, resp.Status)
				continue
			}
			t.Logf("%s found. %s: %s %s: %s",
				test.want,
				headers.ContentType,
				resp.Header.Get(headers.ContentType),
				headers.ContentLength,
				resp.Header.Get(headers.ContentLength),
			)
		}
	})
}
