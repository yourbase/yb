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

package plumbing

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/yourbase/commons/http/headers"
)

func TestDownloadFile(t *testing.T) {
	tests := []struct {
		name      string
		handle    http.HandlerFunc
		want      string
		wantError bool
	}{
		{
			name: "Success",
			handle: func(w http.ResponseWriter, r *http.Request) {
				const content = "Hello, World!\n"
				w.Header().Set(headers.ContentLength, fmt.Sprint(len(content)))
				io.WriteString(w, content)
			},
			want: "Hello, World!\n",
		},
		{
			name: "404",
			handle: func(w http.ResponseWriter, r *http.Request) {
				http.NotFound(w, r)
				io.WriteString(w, "bork")
			},
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			srv := httptest.NewServer(test.handle)
			t.Cleanup(srv.Close)
			dir := t.TempDir()
			dst := filepath.Join(dir, "download.out")

			err := DownloadFile(context.Background(), srv.Client(), dst, srv.URL)

			if err != nil {
				t.Logf("DownloadFile: %v", err)
				if !test.wantError {
					t.Fail()
				}
				if _, err := os.Stat(dst); err == nil {
					t.Errorf("DownloadFile left %s on disk", err)
				} else if !os.IsNotExist(err) {
					t.Error(err)
				}
				return
			}
			if test.wantError {
				t.Errorf("DownloadFile did not return an error")
			}
			data, err := ioutil.ReadFile(dst)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != test.want {
				t.Errorf("content = %q; want %q", data, test.want)
			}
		})
	}
}

func TestValidateDownloadCache(t *testing.T) {
	tests := []struct {
		name      string
		cacheData string
		handle    http.HandlerFunc
		wantError bool
	}{
		{
			name: "NoFile",
			handle: func(w http.ResponseWriter, r *http.Request) {
				const content = "Hello, World!\n"
				w.Header().Set(headers.ContentLength, fmt.Sprint(len(content)))
				io.WriteString(w, content)
			},
			wantError: true,
		},
		{
			name:      "SameData",
			cacheData: "Hello, World!\n",
			handle: func(w http.ResponseWriter, r *http.Request) {
				const content = "Hello, World!\n"
				w.Header().Set(headers.ContentLength, fmt.Sprint(len(content)))
				io.WriteString(w, content)
			},
			wantError: false,
		},
		{
			name:      "DifferentData",
			cacheData: "old",
			handle: func(w http.ResponseWriter, r *http.Request) {
				const content = "batman!\n"
				w.Header().Set(headers.ContentLength, fmt.Sprint(len(content)))
				io.WriteString(w, content)
			},
			wantError: true,
		},
		{
			name:      "SameSizeData",
			cacheData: "123456789",
			handle: func(w http.ResponseWriter, r *http.Request) {
				const content = "987654321"
				w.Header().Set(headers.ContentLength, fmt.Sprint(len(content)))
				io.WriteString(w, content)
			},
			wantError: false,
		},
		{
			name:      "404",
			cacheData: "i haz a bucket",
			handle: func(w http.ResponseWriter, r *http.Request) {
				http.NotFound(w, r)
				io.WriteString(w, "bork")
			},
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			srv := httptest.NewServer(test.handle)
			t.Cleanup(srv.Close)
			dir := t.TempDir()
			cacheFilename := filepath.Join(dir, "cached.dat")
			if test.cacheData != "" {
				if err := ioutil.WriteFile(cacheFilename, []byte(test.cacheData), 0666); err != nil {
					t.Fatal(err)
				}
			}

			err := validateDownloadCache(context.Background(), srv.Client(), cacheFilename, srv.URL)

			if err != nil {
				t.Logf("DownloadFile: %v", err)
				if !test.wantError {
					t.Fail()
				}
				return
			}
			if test.wantError {
				t.Errorf("DownloadFile did not return an error")
			}
		})
	}
}
