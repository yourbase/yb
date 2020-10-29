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

package replay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/yourbase/yb/internal/biome"
)

// A Recorder wraps another biome and records its commands in memory.
// The interactions are saved to the filesystem when it is finished.
type Recorder struct {
	biome biome.Biome
	dir   string

	mu   sync.Mutex
	data *replayData
}

// NewRecorder returns a new Recorder biome that wraps the given biome and
// saves to dir on Close.
func NewRecorder(dir string, bio biome.Biome) *Recorder {
	return &Recorder{
		biome: bio,
		dir:   dir,
		data: &replayData{
			Descriptor:   bio.Describe(),
			Dirs:         bio.Dirs(),
			CleanedPaths: make(map[string]string),
			AbsPaths:     make(map[string]bool),
		},
	}
}

// Close writes the recorded invocations to the directory passed into NewRecorder.
func (rec *Recorder) Close() error {
	rec.mu.Lock()
	fname := recordFilename(rec.data.Descriptor)
	jsonData, err := json.MarshalIndent(rec.data, "", "\t")
	rec.mu.Unlock()
	if err != nil {
		return fmt.Errorf("close recorder: %w", err)
	}
	if err := os.MkdirAll(rec.dir, 0777); err != nil {
		return fmt.Errorf("close recorder: %w", err)
	}
	if err := ioutil.WriteFile(filepath.Join(rec.dir, fname), jsonData, 0666); err != nil {
		return fmt.Errorf("close recorder: %w", err)
	}
	return nil
}

func recordFilename(desc *biome.Descriptor) string {
	return desc.OS + "_" + desc.Arch + ".json"
}

func (rec *Recorder) Describe() *biome.Descriptor {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return rec.data.Descriptor
}

func (rec *Recorder) Dirs() *biome.Dirs {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return rec.data.Dirs
}

func (rec *Recorder) Run(ctx context.Context, invoke *biome.Invocation) error {
	// Intercept I/O and then run in biome.
	forwarded := &biome.Invocation{
		Argv: invoke.Argv,
		Dir:  invoke.Dir,
		Env:  invoke.Env,
	}
	stdinHash := sha256.New()
	if invoke.Stdin != nil {
		forwarded.Stdin = io.TeeReader(invoke.Stdin, stdinHash)
	}
	var stdout, stderr *bytes.Buffer
	var combinedOutput *bytes.Buffer
	if isCombinedOutput(invoke.Stdout, invoke.Stderr) {
		combinedOutput = new(bytes.Buffer)
		forwarded.Stdout = io.MultiWriter(invoke.Stdout, combinedOutput)
		forwarded.Stderr = forwarded.Stdout
	} else {
		if invoke.Stdout != nil {
			stdout = new(bytes.Buffer)
			forwarded.Stdout = io.MultiWriter(invoke.Stdout, stdout)
		}
		if invoke.Stderr != nil {
			stderr = new(bytes.Buffer)
			forwarded.Stderr = io.MultiWriter(invoke.Stderr, stderr)
		}
	}
	err := rec.biome.Run(ctx, forwarded)

	// Save invocation.
	recorded := &invocation{
		Argv: append([]string(nil), invoke.Argv...),
		Dir:  invoke.Dir,
	}
	if len(invoke.Env.Vars) > 0 {
		recorded.EnvVars = make(map[string]string, len(invoke.Env.Vars))
		for k, v := range invoke.Env.Vars {
			recorded.EnvVars[k] = v
		}
	}
	if len(invoke.Env.PrependPath) > 0 {
		recorded.PrependPath = append([]string(nil), invoke.Env.PrependPath...)
	}
	if len(invoke.Env.AppendPath) > 0 {
		recorded.AppendPath = append([]string(nil), invoke.Env.AppendPath...)
	}
	if invoke.Stdin != nil {
		recorded.StdinSHA256 = stdinHash.Sum(nil)
	}
	if combinedOutput != nil {
		recorded.Output = &invocationOutput{
			combined: ensureBytesNotNil(combinedOutput.Bytes()),
		}
	}
	if stdout != nil || stderr != nil {
		recorded.Output = new(invocationOutput)
		if stdout != nil {
			recorded.Output.stdout = ensureBytesNotNil(stdout.Bytes())
		}
		if stderr != nil {
			recorded.Output.stderr = ensureBytesNotNil(stderr.Bytes())
		}
	}
	if err != nil {
		recorded.Error = err.Error()
	}
	rec.mu.Lock()
	rec.data.Invocations = append(rec.data.Invocations, recorded)
	rec.mu.Unlock()

	return err
}

func (rec *Recorder) JoinPath(elem ...string) string {
	got := rec.biome.JoinPath(elem...)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.data.JoinedPaths = append(rec.data.JoinedPaths, &joinPathLookup{
		Elems:  append([]string(nil), elem...),
		Result: got,
	})
	return got
}

func (rec *Recorder) CleanPath(path string) string {
	got := rec.biome.CleanPath(path)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.data.CleanedPaths[path] = got
	return got
}

func (rec *Recorder) IsAbsPath(path string) bool {
	got := rec.biome.IsAbsPath(path)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.data.AbsPaths[path] = got
	return got
}

func ensureBytesNotNil(b []byte) []byte {
	if b == nil {
		return []byte{}
	}
	return b
}

func isCombinedOutput(stdout, stderr io.Writer) bool {
	return stdout != nil && stdout == stderr
}
