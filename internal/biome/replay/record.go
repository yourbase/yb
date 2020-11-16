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
	"hash"
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

	// mu protects the data fields except Descriptor and Dirs, which are
	// both immutable.
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
			Descriptor: bio.Describe(),
			Dirs:       bio.Dirs(),
			AbsPaths:   make(map[string]bool),
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
	return rec.data.Descriptor
}

func (rec *Recorder) Dirs() *biome.Dirs {
	return rec.data.Dirs
}

func (rec *Recorder) Run(ctx context.Context, invoke *biome.Invocation) error {
	wrapped, sinks := captureIO(invoke)
	err := rec.biome.Run(ctx, wrapped)
	recorded := saveInvocation(invoke, sinks, err)
	rec.mu.Lock()
	rec.data.Invocations = append(rec.data.Invocations, recorded)
	rec.mu.Unlock()
	return err
}

type ioSinks struct {
	stdinHash      hash.Hash
	stdout         *bytes.Buffer
	stderr         *bytes.Buffer
	combinedOutput *bytes.Buffer
}

// captureIO creates a new invocation that wraps another by teeing its
// standard I/O streams to the returned sinks.
func captureIO(original *biome.Invocation) (*biome.Invocation, ioSinks) {
	wrapped := new(biome.Invocation)
	*wrapped = *original
	var sinks ioSinks
	if original.Stdin != nil {
		sinks.stdinHash = sha256.New()
		wrapped.Stdin = io.TeeReader(original.Stdin, sinks.stdinHash)
	}
	if isCombinedOutput(original.Stdout, original.Stderr) {
		sinks.combinedOutput = new(bytes.Buffer)
		wrapped.Stdout = io.MultiWriter(original.Stdout, sinks.combinedOutput)
		wrapped.Stderr = wrapped.Stdout
	} else {
		if original.Stdout != nil {
			sinks.stdout = new(bytes.Buffer)
			wrapped.Stdout = io.MultiWriter(original.Stdout, sinks.stdout)
		}
		if original.Stderr != nil {
			sinks.stderr = new(bytes.Buffer)
			wrapped.Stderr = io.MultiWriter(original.Stderr, sinks.stderr)
		}
	}
	return wrapped, sinks
}

func (sinks ioSinks) output() *invocationOutput {
	if sinks.combinedOutput != nil {
		return &invocationOutput{
			combined: ensureBytesNotNil(sinks.combinedOutput.Bytes()),
		}
	}
	if sinks.stdout == nil && sinks.stderr == nil {
		return nil
	}
	out := new(invocationOutput)
	if sinks.stdout != nil {
		out.stdout = ensureBytesNotNil(sinks.stdout.Bytes())
	}
	if sinks.stderr != nil {
		out.stderr = ensureBytesNotNil(sinks.stderr.Bytes())
	}
	return out
}

// saveInvocation converts the result of a Run into a replayable invocation.
func saveInvocation(invoke *biome.Invocation, sinks ioSinks, err error) *invocation {
	recorded := &invocation{
		Argv:   append([]string(nil), invoke.Argv...),
		Dir:    invoke.Dir,
		Output: sinks.output(),
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
	if sinks.stdinHash != nil {
		recorded.StdinSHA256 = sinks.stdinHash.Sum(nil)
	}
	if err != nil {
		recorded.Error = err.Error()
	}
	return recorded
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
