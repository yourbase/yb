// Copyright 2021 YourBase Inc.
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
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zombiezen.com/go/log/testlog"
)

func TestRunCommand(t *testing.T) {
	cfg, err := ioutil.ReadFile(filepath.Join("testdata", "TestRunCmd", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}

	cdTempDir(t)
	if err := ioutil.WriteFile(".yourbase.yml", cfg, 0666); err != nil {
		t.Fatal(err)
	}
	outFile, err := os.Create("out.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer outFile.Close()
	oldStdout := os.Stdout
	os.Stdout = outFile
	t.Cleanup(func() { os.Stdout = oldStdout })

	ctx := testlog.WithTB(context.Background(), t)
	c := newRunCmd()
	const want = "foo\n"
	c.SetArgs([]string{"echo", strings.TrimSuffix(want, "\n")})
	if err := c.ExecuteContext(ctx); err != nil {
		t.Error("yb run:", err)
	}
	if _, err := outFile.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	got, err := ioutil.ReadAll(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("stdout = %q; want %q", got, want)
	}
}
