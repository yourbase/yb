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
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log/testlog"
)

func TestHeroku(t *testing.T) {
	ctx := testlog.WithTB(context.Background(), t)
	herokuBiome, _ := testInstall(ctx, t, "heroku:latest")
	versionOutput := new(strings.Builder)
	err := herokuBiome.Run(ctx, &biome.Invocation{
		Argv:   []string{"heroku", "--version"},
		Stdout: versionOutput,
		Stderr: versionOutput,
	})
	t.Logf("heroku --version output:\n%s", versionOutput)
	if err != nil {
		t.Errorf("heroku --version: %v", err)
	}

	if runtime.GOOS == "darwin" {
		// The Heroku CLI seems to daemonize a version cache on macOS that races
		// with the `rm -rf TEMPDIR`. Wait until that settles before ending the test.
		versionPath := herokuBiome.JoinPath(herokuBiome.Dirs().Home, "Library", "Caches", "heroku", "version")
		for {
			if _, err := biome.EvalSymlinks(ctx, herokuBiome, versionPath); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
