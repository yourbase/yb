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

package biome

import (
	"context"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/yourbase/narwhal"
	"zombiezen.com/go/log/testlog"
)

var _ interface {
	Biome
	fileWriter
	dirMaker
} = new(Container)

func TestContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker test due to -short")
	}

	ctx := testlog.WithTB(context.Background(), t)
	dockerClient, err := docker.NewVersionedClientFromEnv("1.39")
	if err != nil {
		t.Fatal(err)
	}
	if err := dockerClient.Ping(); err != nil {
		t.Skip("Docker not available:", err)
	}
	packageDir := t.TempDir()
	const fname1 = "foo.txt"
	const want1 = "Hello, World!\n"
	if err := ioutil.WriteFile(filepath.Join(packageDir, fname1), []byte(want1), 0666); err != nil {
		t.Fatal(err)
	}
	mountDir := t.TempDir()
	const fname2 = "bar.txt"
	const want2 = "Line from an additional mount...\n"
	if err := ioutil.WriteFile(filepath.Join(mountDir, fname2), []byte(want2), 0666); err != nil {
		t.Fatal(err)
	}
	tiniResp, err := http.Get(TiniURL)
	if err != nil {
		t.Fatal(err)
	}
	defer tiniResp.Body.Close()
	pullOutput := new(strings.Builder)

	c, err := CreateContainer(ctx, dockerClient, &ContainerOptions{
		PackageDir: packageDir,
		HomeDir:    t.TempDir(),
		TiniExe:    tiniResp.Body,
		PullOutput: pullOutput,
		Definition: &narwhal.ContainerDefinition{
			Mounts: []docker.HostMount{
				{
					Source: mountDir,
					Target: "/mymount",
					Type:   BindMount,
				},
			},
		},
	})
	if pullOutput.Len() > 0 {
		t.Logf("Pull:\n%s", pullOutput)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error("Close:", err)
		}
	})

	stdout := new(strings.Builder)
	stderr := new(strings.Builder)
	err = c.Run(ctx, &Invocation{
		Argv:   []string{"cat", fname1, "/mymount/" + fname2},
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		t.Error("Run:", err)
	}
	if stderr.Len() > 0 {
		t.Logf("stderr:\n%s", stderr)
	}
	const want = want1 + want2
	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q; want %q", got, want)
	}
}
