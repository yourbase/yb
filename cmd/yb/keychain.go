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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log"
)

// ensureKeychain ensures that a default keychain is present in the biome.
// If the biome is not a macOS environment, then ensureKeychain does nothing.
func ensureKeychain(ctx context.Context, bio biome.Biome) error {
	if bio.Describe().OS != biome.MacOS {
		return nil
	}

	// Check whether a default keychain already exists.
	stdout := new(strings.Builder)
	stderr := new(strings.Builder)
	err := bio.Run(ctx, &biome.Invocation{
		Argv:   []string{"security", "default-keychain", "-d", "user"},
		Stdout: stdout,
		Stderr: stderr,
	})
	if err == nil && len(parseKeychainOutput(stdout.String())) > 0 {
		// Keychain already exists.
		return nil
	}
	if err != nil {
		if stderr.Len() > 0 {
			log.Debugf(ctx, "No default keychain; will create. Error:\n%s%v", stderr, err)
		} else {
			log.Debugf(ctx, "No default keychain; will create. Error: %v", err)
		}
	}

	// From experimentation, `security list-keychains -s` will silently fail
	// unless ~/Library/Preferences exists.
	if err := biome.MkdirAll(ctx, bio, bio.JoinPath(bio.Dirs().Home, "Library", "Preferences")); err != nil {
		return fmt.Errorf("ensure build environment keychain: %w", err)
	}

	// List the existing user keychains. There likely won't be any, but for
	// robustness, we preserve them in the search path.
	stdout.Reset()
	stderr.Reset()
	err = bio.Run(ctx, &biome.Invocation{
		Argv:   []string{"security", "list-keychains", "-d", "user"},
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		if stderr.Len() > 0 {
			// stderr will almost certainly end in '\n'.
			return fmt.Errorf("ensure build environment keychain: %s%w", stderr, err)
		}
		return fmt.Errorf("ensure build environment keychain: %w", err)
	}
	keychainList := parseKeychainOutput(stdout.String())

	// Create a passwordless keychain.
	const keychainName = "login.keychain"
	if err := runCommand(ctx, bio, "security", "create-keychain", "-p", "", keychainName); err != nil {
		return fmt.Errorf("ensure build environment keychain: %w", err)
	}

	// The keychain must be added to the search path.
	// See https://stackoverflow.com/questions/20391911/os-x-keychain-not-visible-to-keychain-access-app-in-mavericks
	//
	// We prepend it to the search path so that Fastlane picks it up:
	// https://github.com/fastlane/fastlane/blob/832e3e4a19d9cff5d5a14a61e9614b5659327427/fastlane_core/lib/fastlane_core/cert_checker.rb#L133-L134
	searchPathArgs := []string{"security", "list-keychains", "-d", "user", "-s", keychainName}
	for _, k := range keychainList {
		searchPathArgs = append(searchPathArgs, filepath.Base(k))
	}
	if err := runCommand(ctx, bio, searchPathArgs...); err != nil {
		return fmt.Errorf("ensure build environment keychain: %w", err)
	}

	// Set the new keychain as the default.
	if err := runCommand(ctx, bio, "security", "default-keychain", "-s", keychainName); err != nil {
		return fmt.Errorf("ensure build environment keychain: %w", err)
	}
	return nil
}

func parseKeychainOutput(out string) []string {
	lines := strings.Split(out, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, `"`) || !strings.HasSuffix(line, `"`) {
			continue
		}
		paths = append(paths, line[1:len(line)-1])
	}
	return paths
}
