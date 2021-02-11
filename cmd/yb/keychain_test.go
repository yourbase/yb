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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log/testlog"
)

func TestEnsureKeychain(t *testing.T) {
	ctx := testlog.WithTB(context.Background(), t)
	l := biome.Local{
		PackageDir: t.TempDir(),
		HomeDir:    t.TempDir(),
	}
	if osName := l.Describe().OS; osName != biome.MacOS {
		t.Skipf("OS = %q; test can only run on %q", osName, biome.MacOS)
	}
	if err := ensureKeychain(ctx, l); err != nil {
		t.Error("ensureKeychain(...):", err)
	}

	// Verify that a single keychain shows up in the list.
	stdout := new(strings.Builder)
	stderr := new(strings.Builder)
	err := l.Run(ctx, &biome.Invocation{
		Argv:   []string{"security", "list-keychains", "-d", "user"},
		Stdout: stdout,
		Stderr: stderr,
	})
	if stderr.Len() > 0 {
		t.Logf("stderr:\n%s", stderr)
	}
	if err != nil {
		t.Error("security list-keychains:", err)
	}
	if keychains := parseKeychainOutput(stdout.String()); len(keychains) != 1 {
		t.Errorf("keychains = %q; want len(keychains) == 1", keychains)
	}

	// Verify that a default keychain is set.
	stdout.Reset()
	stderr.Reset()
	err = l.Run(ctx, &biome.Invocation{
		Argv:   []string{"security", "default-keychain", "-d", "user"},
		Stdout: stdout,
		Stderr: stderr,
	})
	if stderr.Len() > 0 {
		t.Logf("stderr:\n%s", stderr)
	}
	if err != nil {
		t.Error("security default-keychain:", err)
	}
	if len(parseKeychainOutput(stdout.String())) == 0 {
		t.Error("No default keychain set")
	}

	// Verify that running multiple times does not return an error and does not
	// create a new keychain.
	if err := ensureKeychain(ctx, l); err != nil {
		t.Error("Second ensureKeychain(...):", err)
	}
	stdout.Reset()
	stderr.Reset()
	err = l.Run(ctx, &biome.Invocation{
		Argv:   []string{"security", "list-keychains", "-d", "user"},
		Stdout: stdout,
		Stderr: stderr,
	})
	if stderr.Len() > 0 {
		t.Logf("stderr:\n%s", stderr)
	}
	if err != nil {
		t.Error("security list-keychains:", err)
	}
	if keychains := parseKeychainOutput(stdout.String()); len(keychains) != 1 {
		t.Errorf("keychains after second ensure = %q; want len(keychains) == 1", keychains)
	}
}

func TestParseKeychainOutput(t *testing.T) {
	tests := []struct {
		output string
		want   []string
	}{
		{
			output: "",
			want:   nil,
		},
		{
			output: "    \"/Users/yourbase/Library/Keychains/login.keychain-db\"\n",
			want:   []string{"/Users/yourbase/Library/Keychains/login.keychain-db"},
		},
		{
			output: "    \"/Users/yourbase/Library/Keychains/login.keychain-db\"\n" +
				"    \"/Library/Keychains/System.keychain\"\n",
			want: []string{
				"/Users/yourbase/Library/Keychains/login.keychain-db",
				"/Library/Keychains/System.keychain",
			},
		},
	}
	for _, test := range tests {
		got := parseKeychainOutput(test.output)
		if diff := cmp.Diff(test.want, got, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("parseKeychainOutput(%q) (-want +got):\n%s", test.output, diff)
		}
	}
}
