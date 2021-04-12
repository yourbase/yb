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

// Package remote provides a client for a remote biome.
package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	slashpath "path"
	"strconv"
	"strings"
	"time"

	"github.com/jpillora/backoff"
	"github.com/yourbase/commons/http/headers"
	"github.com/yourbase/yb/internal/biome"
	"golang.org/x/sync/errgroup"
	"zombiezen.com/go/log"
)

type Biome struct {
	client *http.Client
	base   *url.URL
	desc   biome.Descriptor
	dirs   biome.Dirs
}

func Connect(ctx context.Context, client *http.Client, base *url.URL, packageDir string) (*Biome, error) {
	b := &Biome{
		client: client,
		base:   base,
	}
	if err := b.fillInfo(ctx); err != nil {
		return nil, fmt.Errorf("connect remote biome %v: %w", base, err)
	}
	if err := b.uploadPackage(ctx, packageDir); err != nil {
		return nil, fmt.Errorf("connect remote biome %v: %w", base, err)
	}
	return b, nil
}

func (b *Biome) fillInfo(ctx context.Context) error {
	req := &http.Request{
		Method: http.MethodGet,
		URL:    b.base,
		Body:   http.NoBody,
	}
	resp, err := do(b.client, req.WithContext(ctx), "application/json")
	if err != nil {
		return fmt.Errorf("get info: %w", err)
	}
	var info struct {
		Descriptor struct {
			OS   string
			Arch string
		}
		Dirs struct {
			Package string
			Home    string
			Tools   string
		}
	}
	if err := readJSONBody(&info, resp.Body); err != nil {
		return fmt.Errorf("get info: %w", err)
	}
	b.desc.OS = info.Descriptor.OS
	b.desc.Arch = info.Descriptor.Arch
	b.dirs.Package = info.Dirs.Package
	b.dirs.Home = info.Dirs.Home
	b.dirs.Tools = info.Dirs.Tools
	return nil
}

func (b *Biome) uploadPackage(ctx context.Context, dir string) error {
	// TODO(soon): Create tar in-process.
	tarCmd := exec.CommandContext(ctx, "tar", "-c", "-f", "-", ".")
	tarCmd.Dir = dir
	tarCmd.Stderr = os.Stderr
	tarStream, err := tarCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("upload package: %w", err)
	}
	if err := tarCmd.Start(); err != nil {
		return fmt.Errorf("upload package: %w", err)
	}
	defer func() {
		tarStream.Close()
		tarCmd.Process.Kill()
		tarCmd.Wait()
	}()
	req := &http.Request{
		Method: http.MethodPut,
		URL:    addURLPath(b.base, "/package"),
		Body:   tarStream,
		Header: http.Header{
			headers.ContentType: {"application/x-tar"},
		},
	}
	resp, err := do(b.client, req.WithContext(ctx), "")
	if err != nil {
		return fmt.Errorf("upload package: %w", err)
	}
	resp.Body.Close()
	return nil
}

func (b *Biome) Run(ctx context.Context, invoke *biome.Invocation) error {
	outputMode := ""
	switch {
	case invoke.Stdout != nil && invoke.Stdout == invoke.Stderr:
		outputMode = "combined"
	case invoke.Stdout != nil && invoke.Stderr != nil:
		outputMode = "all"
	case invoke.Stdout != nil:
		outputMode = "stdout"
	case invoke.Stderr != nil:
		outputMode = "stderr"
	}
	startBody, err := json.Marshal(map[string]interface{}{
		"argv": invoke.Argv,
		"dir":  invoke.Dir,
		"env": map[string]interface{}{
			"vars":         invoke.Env.Vars,
			"append_path":  invoke.Env.AppendPath,
			"prepend_path": invoke.Env.PrependPath,
		},
		"attach_stdin": invoke.Stdin != nil,
		"output_mode":  outputMode,
	})
	if err != nil {
		return fmt.Errorf("run remote: %w", err)
	}
	startRequest := &http.Request{
		Method: http.MethodPost,
		URL:    addURLPath(b.base, "/commands/"),
		Header: http.Header{
			headers.ContentType:   {"application/json; charset=utf-8"},
			headers.ContentLength: {strconv.Itoa(len(startBody))},
		},
		GetBody: func() (io.ReadCloser, error) {
			return ioutil.NopCloser(bytes.NewReader(startBody)), nil
		},
	}
	startRequest.Body, _ = startRequest.GetBody()
	startResponse, err := do(b.client, startRequest.WithContext(ctx), "application/json")
	if err != nil {
		return fmt.Errorf("run remote: %w", err)
	}
	var command struct {
		ID string
	}
	if err := readJSONBody(&command, startResponse.Body); err != nil {
		return fmt.Errorf("run remote: %w", err)
	}

	grp, grpCtx := errgroup.WithContext(ctx)
	if invoke.Stdin != nil {
		grp.Go(func() error {
			return b.copyInputStream(grpCtx, fmt.Sprintf("/commands/%s/stdin", command.ID), invoke.Stdin)
		})
	}
	if outputMode == "combined" || outputMode == "all" || outputMode == "stdout" {
		grp.Go(func() error {
			return b.copyOutputStream(grpCtx, invoke.Stdout, fmt.Sprintf("/commands/%s/stdout", command.ID))
		})
	}
	if outputMode == "all" || outputMode == "stderr" {
		grp.Go(func() error {
			return b.copyOutputStream(grpCtx, invoke.Stderr, fmt.Sprintf("/commands/%s/stderr", command.ID))
		})
	}
	var result *commandResult
	grp.Go(func() error {
		var err error
		result, err = b.waitForCommand(ctx, command.ID)
		return err
	})
	if err := grp.Wait(); err != nil {
		return fmt.Errorf("run remote: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("run remote: %s", result.ErrorMessage)
	}
	return nil
}

type commandResult struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error,omitempty"`
}

func (b *Biome) waitForCommand(ctx context.Context, commandID string) (*commandResult, error) {
	backoffStrategy := &backoff.Backoff{
		Factor: 2.0,
		Jitter: true,
		Min:    5 * time.Millisecond,
		Max:    5 * time.Second,
	}
	t := time.NewTimer(backoffStrategy.Duration())
	defer t.Stop()
	for {
		select {
		case <-t.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		req := &http.Request{
			Method: http.MethodGet,
			URL:    addURLPath(b.base, fmt.Sprintf("/commands/%s/", commandID)),
			Header: http.Header{
				headers.Accept: {"application/json"},
			},
		}
		resp, err := do(b.client, req, "application/json")
		if err != nil {
			d := backoffStrategy.Duration()
			log.Errorf(ctx, "While waiting for command to %q finish: %v (will retry in %v)", commandID, err, d)
			t.Reset(d)
			continue
		}
		var refreshedCommand struct {
			Result *commandResult
		}
		if err := readJSONBody(&refreshedCommand, resp.Body); err != nil {
			d := backoffStrategy.Duration()
			log.Errorf(ctx, "While waiting for command %q to finish: %v (will retry in %v)", commandID, err, d)
			t.Reset(d)
			continue
		}
		if refreshedCommand.Result != nil {
			return refreshedCommand.Result, nil
		}

		d := backoffStrategy.Duration()
		log.Debugf(ctx, "Command %q not finished yet (will retry in %v)", commandID, d)
		t.Reset(d)
	}
}

func (b *Biome) copyInputStream(ctx context.Context, path string, src io.Reader) error {
	req := &http.Request{
		Method: http.MethodPost,
		URL:    addURLPath(b.base, path),
		Body:   ioutil.NopCloser(src),
		Header: http.Header{
			headers.ContentType: {"application/octet-stream"},
		},
	}
	req.URL.RawQuery = "close=1"
	resp, err := do(b.client, req.WithContext(ctx), "")
	if err != nil {
		return fmt.Errorf("copy input stream: %w", err)
	}
	resp.Body.Close()
	return nil
}

func (b *Biome) copyOutputStream(ctx context.Context, dst io.Writer, path string) error {
	req := &http.Request{
		Method: http.MethodGet,
		URL:    addURLPath(b.base, path),
		Header: http.Header{
			headers.Accept: {"application/octet-stream"},
		},
	}
	resp, err := do(b.client, req.WithContext(ctx), "")
	if err != nil {
		return fmt.Errorf("copy output stream: %w", err)
	}
	if _, err := io.Copy(dst, resp.Body); err != nil {
		return fmt.Errorf("copy output stream: %w", err)
	}
	return nil
}

func (b *Biome) Describe() *biome.Descriptor {
	return &b.desc
}

func (b *Biome) Dirs() *biome.Dirs {
	return &b.dirs
}

func (b *Biome) JoinPath(elem ...string) string {
	// TODO(light): If b.desc.OS == biome.Windows, use \-separated paths.
	return slashpath.Join(elem...)
}

func (b *Biome) IsAbsPath(path string) bool {
	// TODO(light): If b.desc.OS == biome.Windows, use \-separated paths.
	return slashpath.IsAbs(path)
}

func do(client *http.Client, req *http.Request, contentType string) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if !(200 <= resp.StatusCode && resp.StatusCode < 300) {
		resp.Body.Close()
		return nil, fmt.Errorf("http %s", resp.Status)
	}
	if contentType != "" {
		// TODO(soon): Swap out for parsing request Accept header.
		gotContentType := resp.Header.Get(headers.ContentType)
		if got, _, err := mime.ParseMediaType(gotContentType); err != nil || got != contentType {
			resp.Body.Close()
			return nil, fmt.Errorf("received %q (expected %q)", gotContentType, contentType)
		}
	}
	return resp, nil
}

func readJSONBody(dst interface{}, rc io.ReadCloser) error {
	data, err := ioutil.ReadAll(rc)
	rc.Close()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

func addURLPath(u *url.URL, path string) *url.URL {
	return &url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
		User:   u.User,
		Path:   strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(path, "/"),
	}
}
