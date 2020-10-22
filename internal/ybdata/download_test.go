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

package ybdata

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
	"zombiezen.com/go/log/testlog"
)

func TestDownload(t *testing.T) {
	tests := []struct {
		name         string
		handle       http.HandlerFunc
		want         string
		wantError    bool
		wantNotFound bool
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
			},
			wantError:    true,
			wantNotFound: true,
		},
		{
			name: "500",
			handle: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "bork", http.StatusInternalServerError)
			},
			wantError:    true,
			wantNotFound: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			srv := httptest.NewServer(test.handle)
			t.Cleanup(srv.Close)
			dataDirs := NewDirs(t.TempDir())

			f, err := Download(context.Background(), srv.Client(), dataDirs, srv.URL)
			if err != nil {
				t.Logf("download: %v", err)
				if !test.wantError {
					t.Fail()
				}
				if got := IsNotFound(err); got != test.wantNotFound {
					t.Errorf("is not found error = %t; want %t", got, test.wantNotFound)
				}
				files, err := ioutil.ReadDir(dataDirs.Downloads())
				if err != nil && !os.IsNotExist(err) {
					t.Error(err)
				}
				if len(files) > 0 {
					t.Errorf("download left %s on disk", files)
				}
				return
			}
			defer f.Close()

			if test.wantError {
				t.Errorf("download did not return an error")
				return
			}
			data, err := ioutil.ReadAll(f)
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
		name         string
		cacheData    string
		handle       http.HandlerFunc
		wantError    bool
		wantNotFound bool
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
			},
			wantError:    true,
			wantNotFound: true,
		},
		{
			name:      "500",
			cacheData: "i haz a bucket",
			handle: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "bork", http.StatusInternalServerError)
			},
			wantError:    true,
			wantNotFound: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			srv := httptest.NewServer(test.handle)
			t.Cleanup(srv.Close)
			dir := t.TempDir()
			f, err := os.Create(filepath.Join(dir, "cached.dat"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.WriteString(test.cacheData); err != nil {
				t.Fatal(err)
			}

			err = validateDownloadCache(context.Background(), srv.Client(), f, srv.URL)
			if err != nil {
				t.Logf("validateDownloadCache: %v", err)
				if !test.wantError {
					t.Fail()
				}
				if got := IsNotFound(err); got != test.wantNotFound {
					t.Errorf("is not found error = %t; want %t", got, test.wantNotFound)
				}
				return
			}
			defer f.Close()

			if test.wantError {
				t.Errorf("validateDownloadCache did not return an error")
			}
		})
	}
}

func TestMain(m *testing.M) {
	testlog.Main(nil)
	os.Exit(m.Run())
}
