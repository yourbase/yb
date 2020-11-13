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

// Package replay provides a biome that records all events to an underlying
// biome and then allows replaying those command invocations later.
package replay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yourbase/commons/jsonstring"
	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log"
)

// replayData is the JSON document saved by a Recorder.
type replayData struct {
	Descriptor  *biome.Descriptor
	Dirs        *biome.Dirs
	JoinedPaths []*joinPathLookup
	AbsPaths    map[string]bool
	Invocations []*invocation
}

type joinPathLookup struct {
	Elems  []string
	Result string
}

// invocation is the result of a single command run.
type invocation struct {
	Argv        []string
	Dir         string            `json:",omitempty"`
	EnvVars     map[string]string `json:",omitempty"`
	PrependPath []string          `json:",omitempty"`
	AppendPath  []string          `json:",omitempty"`
	StdinSHA256 hexString         `json:",omitempty"`
	Output      *invocationOutput `json:",omitempty"`
	Error       string            `json:",omitempty"`
}

func (i *invocation) env() biome.Environment {
	return biome.Environment{
		Vars:        i.EnvVars,
		AppendPath:  i.AppendPath,
		PrependPath: i.PrependPath,
	}
}

type invocationOutput struct {
	combined []byte
	stdout   []byte
	stderr   []byte
}

func (out *invocationOutput) MarshalJSON() ([]byte, error) {
	if out == nil {
		return []byte("null"), nil
	}
	var buf []byte
	buf = append(buf, '{')
	if out.combined != nil {
		buf = append(buf, `"Combined":`...)
		buf = jsonstring.Append(buf, base64.StdEncoding.EncodeToString(out.combined))
		buf = append(buf, '}')
		return buf, nil
	}

	if out.stdout != nil {
		buf = append(buf, `"Stdout":`...)
		buf = jsonstring.Append(buf, base64.StdEncoding.EncodeToString(out.stdout))
		if out.stderr != nil {
			buf = append(buf, ',')
		}
	}
	if out.stderr != nil {
		buf = append(buf, `"Stderr":`...)
		buf = jsonstring.Append(buf, base64.StdEncoding.EncodeToString(out.stderr))
	}
	buf = append(buf, '}')
	return buf, nil
}

func (out *invocationOutput) UnmarshalJSON(b []byte) error {
	var temp struct {
		Combined *string
		Stdout   *string
		Stderr   *string
	}
	if err := json.Unmarshal(b, &temp); err != nil {
		return err
	}
	if temp.Combined != nil {
		var err error
		out.combined, err = base64.StdEncoding.DecodeString(*temp.Combined)
		if err != nil {
			return fmt.Errorf("decode combined output: %w", err)
		}
		return nil
	}

	if temp.Stdout != nil {
		var err error
		out.stdout, err = base64.StdEncoding.DecodeString(*temp.Stdout)
		if err != nil {
			return fmt.Errorf("decode stdout: %w", err)
		}
	}
	if temp.Stderr != nil {
		var err error
		out.stderr, err = base64.StdEncoding.DecodeString(*temp.Stderr)
		if err != nil {
			return fmt.Errorf("decode stderr: %w", err)
		}
	}
	return nil
}

// Replay is a biome that replays Run commands from a recorded file.
type Replay struct {
	data replayData

	mu          sync.Mutex
	invokeIndex int
	failed      bool
}

// Load reads the biome interactions in the given directory for the given
// descriptor and returns a biome that repeats those commands.
func Load(dir string, desc *biome.Descriptor) (*Replay, error) {
	jsonData, err := ioutil.ReadFile(filepath.Join(dir, recordFilename(desc)))
	if err != nil {
		return nil, fmt.Errorf("load replay for %s (%s/%s): %w", dir, desc.OS, desc.Arch, err)
	}
	r := new(Replay)
	if err := json.Unmarshal(jsonData, &r.data); err != nil {
		return nil, fmt.Errorf("load replay for %s (%s/%s): %w", dir, desc.OS, desc.Arch, err)
	}
	return r, nil
}

// Describe returns the recorded descriptor.
func (r *Replay) Describe() *biome.Descriptor {
	return r.data.Descriptor
}

// Dirs returns the directories used in the recording.
func (r *Replay) Dirs() *biome.Dirs {
	return r.data.Dirs
}

// Run attempts to use the next invocation in the replay sequence, returning an
// error if the invocation parameters don't match. Once Run encounters a
// mismatched invocation, subsequent calls to Run will fail.
func (r *Replay) Run(ctx context.Context, invoke *biome.Invocation) error {
	// Find invocation to replay.
	invokeLine := strings.Join(invoke.Argv, " ")
	r.mu.Lock()
	if r.failed {
		r.mu.Unlock()
		return fmt.Errorf("run replay: `%s`: replay aborted due to previous failure", invokeLine)
	}
	if r.invokeIndex >= len(r.data.Invocations) {
		r.failed = true
		r.mu.Unlock()
		return fmt.Errorf("run replay: `%s`: ran out of recorded commands", invokeLine)
	}
	recorded := r.data.Invocations[r.invokeIndex]
	if !stringSliceEqual(invoke.Argv, recorded.Argv) {
		r.failed = true
		r.mu.Unlock()
		return fmt.Errorf("run replay: `%s`: expected `%s` at index %d", invokeLine, strings.Join(recorded.Argv, " "), r.invokeIndex)
	}
	r.invokeIndex++
	r.mu.Unlock()

	// Replay recorded invocation.
	log.Debugf(ctx, "Replay run: %s", invokeLine)
	if invoke.Stdin == nil {
		if len(recorded.StdinSHA256) > 0 {
			return fmt.Errorf("run replay: `%s`: stdin not provided", invokeLine)
		}
	} else {
		if len(recorded.StdinSHA256) == 0 {
			return fmt.Errorf("run replay: `%s`: stdin provided but not expected", invokeLine)
		}
		h := sha256.New()
		if _, err := io.Copy(h, invoke.Stdin); err != nil {
			return fmt.Errorf("run replay: `%s`: read stdin: %w", invokeLine, err)
		}
		if got := h.Sum(nil); !bytes.Equal(got, recorded.StdinSHA256) {
			return fmt.Errorf("run replay: `%s`: read stdin: does not match recorded input", invokeLine)
		}
	}
	if want := recorded.env(); !envEqual(invoke.Env, want) {
		return fmt.Errorf("run replay: `%s`: environment:\n%v\n; expected:\n%s",
			invokeLine, invoke.Env, want)
	}
	if recorded.Output == nil {
		if invoke.Stdout != nil || invoke.Stderr != nil {
			return fmt.Errorf("run replay: `%s`: output requested but recording does not include output", invokeLine)
		}
	} else {
		if isCombinedOutput(invoke.Stdout, invoke.Stderr) {
			if recorded.Output.combined == nil {
				return fmt.Errorf("run replay: `%s`: combined output requested but recording has separated output", invokeLine)
			}
			if _, err := invoke.Stdout.Write(recorded.Output.combined); err != nil {
				return fmt.Errorf("run replay: `%s`: write combined output: %w", invokeLine, err)
			}
		} else if recorded.Output.combined != nil {
			return fmt.Errorf("run replay: `%s`: separated output requested but recording has combined output", invokeLine)
		} else {
			if invoke.Stdout != nil {
				if recorded.Output.stdout == nil {
					return fmt.Errorf("run replay: `%s`: stdout requested but recording does not have stdout", invokeLine)
				}
				if _, err := invoke.Stdout.Write(recorded.Output.stdout); err != nil {
					return fmt.Errorf("run replay: `%s`: write stdout: %w", invokeLine, err)
				}
			}
			if invoke.Stderr != nil {
				if recorded.Output.stderr == nil {
					return fmt.Errorf("run replay: `%s`: stderr requested but recording does not have stderr", invokeLine)
				}
				if _, err := invoke.Stderr.Write(recorded.Output.stderr); err != nil {
					return fmt.Errorf("run replay: `%s`: write stderr: %w", invokeLine, err)
				}
			}
		}
	}
	if recorded.Error != "" {
		return errors.New(recorded.Error)
	}
	return nil
}

func (r *Replay) JoinPath(elem ...string) string {
	if len(elem) == 0 {
		return ""
	}
	for _, possible := range r.data.JoinedPaths {
		if stringSliceEqual(possible.Elems, elem) {
			return possible.Result
		}
	}
	return "__UNKNOWN_JOIN_PATH__"
}

func (r *Replay) IsAbsPath(path string) bool {
	absPath, ok := r.data.AbsPaths[path]
	if !ok {
		return false
	}
	return absPath
}

type hexString []byte

func (h hexString) MarshalText() ([]byte, error) {
	buf := make([]byte, hex.EncodedLen(len(h)))
	hex.Encode(buf, h)
	return buf, nil
}

func (h *hexString) UnmarshalText(text []byte) error {
	*h = make([]byte, hex.DecodedLen(len(text)))
	_, err := hex.Decode(*h, text)
	return err
}

func (h hexString) String() string {
	return hex.EncodeToString(h)
}

func envEqual(e1, e2 biome.Environment) bool {
	return mapEqual(e1.Vars, e2.Vars) &&
		stringSliceEqual(e1.PrependPath, e2.PrependPath) &&
		stringSliceEqual(e1.AppendPath, e2.AppendPath)
}

func stringSliceEqual(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := range s1 {
		if s1[i] != s2[i] {
			return false
		}
	}
	return true
}

func mapEqual(m1, m2 map[string]string) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k, v1 := range m1 {
		if v2, ok := m2[k]; !ok || v1 != v2 {
			return false
		}
	}
	return true
}
