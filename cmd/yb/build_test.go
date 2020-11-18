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

package main

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"zombiezen.com/go/log/testlog"
)

func TestBuildCmd(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		cfg, err := ioutil.ReadFile(filepath.Join("testdata", "TestBuildCmd", "success.yml"))
		if err != nil {
			t.Fatal(err)
		}

		cdTempDir(t)
		if err := ioutil.WriteFile(".yourbase.yml", cfg, 0666); err != nil {
			t.Fatal(err)
		}

		ctx := testlog.WithTB(context.Background(), t)
		c := newBuildCmd()
		c.SetArgs([]string{"--no-container"})
		if err := c.ExecuteContext(ctx); err != nil {
			t.Error("yb build:", err)
		}
	})

	t.Run("Failure", func(t *testing.T) {
		cfg, err := ioutil.ReadFile(filepath.Join("testdata", "TestBuildCmd", "failure.yml"))
		if err != nil {
			t.Fatal(err)
		}

		cdTempDir(t)
		if err := ioutil.WriteFile(".yourbase.yml", cfg, 0666); err != nil {
			t.Fatal(err)
		}

		ctx := testlog.WithTB(context.Background(), t)
		c := newBuildCmd()
		c.SetArgs([]string{"--no-container"})
		err = c.ExecuteContext(ctx)
		if err == nil {
			t.Fatal("yb build succeeded")
		}
		t.Log("yb build:", err)
	})
}

func cdTempDir(t *testing.T) {
	t.Helper()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chdir(oldWD)
	})
}

func TestMain(m *testing.M) {
	testlog.Main(nil)
	os.Exit(m.Run())
}
