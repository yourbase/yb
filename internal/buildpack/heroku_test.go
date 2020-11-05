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
	"strings"
	"testing"

	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log/testlog"
)

func TestHeroku(t *testing.T) {
	ctx := testlog.WithTB(context.Background(), t)

	// TODO(light): This test is not reproducible, since there's no ability to pin
	// to a specific Heroku version. Instead, we just always run this test without
	// record/replay.
	bio := newLocalTestBiome(t)
	if bio.Describe().OS == biome.MacOS {
		// The Heroku CLI seems to daemonize a version cache on macOS that races
		// with the `rm -rf TEMPDIR`.
		t.Skip("Heroku tests don't clean up properly on macOS. Skipping.")
	}
	herokuEnv, err := runTestInstall(ctx, t, bio, "heroku:latest")
	if err != nil {
		t.Fatal(err)
	}
	herokuBiome := biome.EnvBiome{
		Biome: bio,
		Env:   herokuEnv,
	}

	versionOutput := new(strings.Builder)
	err = herokuBiome.Run(ctx, &biome.Invocation{
		Argv:   []string{"heroku", "--version"},
		Stdout: versionOutput,
		Stderr: versionOutput,
	})
	t.Logf("heroku --version output:\n%s", versionOutput)
	if err != nil {
		t.Errorf("heroku --version: %v", err)
	}
}
