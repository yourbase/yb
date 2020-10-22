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

package ybdata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/yourbase/yb/internal/ybtrace"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
	"zombiezen.com/go/log"
)

func download(ctx context.Context, client *http.Client, dst io.Writer, url string) (err error) {
	ctx, span := ybtrace.Start(ctx, "Download "+url,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			label.String("http.method", http.MethodGet),
			label.String("http.url", url),
		),
	)
	defer func() {
		if err != nil {
			span.SetStatus(codes.Unknown, err.Error())
		}
		span.End()
	}()

	// Make HTTP request.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	log.Infof(ctx, "Downloading %s", url)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	span.SetAttribute("http.status_code", resp.StatusCode)
	span.SetAttribute("http.response_content_length", resp.ContentLength)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %w", url, httpError{
			status:     resp.Status,
			statusCode: resp.StatusCode,
		})
	}

	// Copy to file.
	if _, err := io.Copy(dst, resp.Body); err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	return nil
}

// IsNotFound reports whether e indicates an HTTP 404 Not Found or
// 410 Gone response.
func IsNotFound(e error) bool {
	var httpErr httpError
	if !errors.As(e, &httpErr) {
		return false
	}
	return httpErr.statusCode == http.StatusNotFound ||
		httpErr.statusCode == http.StatusGone
}

type httpError struct {
	status     string
	statusCode int
}

func (e httpError) Error() string {
	return "http " + e.status
}

// Download downloads a URL to the local filesystem and returns a handle to
// the file. Download maintains a cache in dataDirs. If the URL could not be
// found on the server, then IsNotFound(err) will return true.
func Download(ctx context.Context, client *http.Client, dataDirs *Dirs, url string) (_ *os.File, err error) {
	cacheFilename := filepath.Join(dataDirs.Downloads(), cacheFilenameForURL(url))
	if err := os.MkdirAll(filepath.Dir(cacheFilename), 0777); err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	// Create file first, since that requires less work to fail faster.
	f, err := os.OpenFile(cacheFilename, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer func() {
		if err != nil {
			f.Close()
			// If there's an error, the cache has been made inconsistent because
			// we've truncated or created the file. Remove the file to force a
			// download later.
			if err := os.Remove(cacheFilename); err != nil {
				log.Warnf(ctx, "Failed to clean up failed download: %v", err)
			}
		}
	}()

	cacheErr := validateDownloadCache(ctx, client, f, url)
	if cacheErr == nil {
		log.Infof(ctx, "Reusing cached version of %s", url)
		return f, nil
	}
	if IsNotFound(cacheErr) {
		return nil, fmt.Errorf("download %s: %w", url, cacheErr)
	}
	log.Debugf(ctx, "Cache error: %v", cacheErr)
	log.Infof(ctx, "Not using cache for %s", url)
	if err := f.Truncate(0); err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	if err := download(ctx, client, f, url); err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	return f, nil
}

// DownloadFileWithCache downloads a URL to the local filesystem and returns
// the path to the downloaded copy.
//
// Deprecated: Use Download to avoid closing and reopening file.
func DownloadFileWithCache(ctx context.Context, client *http.Client, dataDirs *Dirs, url string) (string, error) {
	f, err := Download(ctx, client, dataDirs, url)
	if err != nil {
		return "", err
	}
	path := f.Name()
	f.Close()
	return path, nil
}

func validateDownloadCache(ctx context.Context, client *http.Client, statter interface{ Stat() (os.FileInfo, error) }, url string) (err error) {
	ctx, span := ybtrace.Start(ctx, "Validate cache for "+url,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			label.String("http.method", http.MethodHead),
			label.String("http.url", url),
		),
	)
	defer func() {
		if err != nil {
			span.SetStatus(codes.Unknown, err.Error())
		}
		span.End()
	}()

	info, err := statter.Stat()
	if err != nil {
		return fmt.Errorf("validate %s download cache: %w", url, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return fmt.Errorf("validate %s download cache: %w", url, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("validate %s download cache: %w", url, err)
	}
	resp.Body.Close()
	span.SetAttribute("http.status_code", resp.StatusCode)
	span.SetAttribute("http.response_content_length", resp.ContentLength)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("validate %s download cache: %w", url, httpError{
			status:     resp.Status,
			statusCode: resp.StatusCode,
		})
	}
	if fileSize := info.Size(); fileSize != resp.ContentLength {
		return fmt.Errorf("validate %s download cache: size %d does not match resource size %d", url, fileSize, resp.ContentLength)
	}
	return nil
}

var cacheFilenameUnsafeChars = regexp.MustCompile(`[^a-zA-Z0-9.]+`)

func cacheFilenameForURL(url string) string {
	// TODO(light): Use a hash-based scheme.
	return cacheFilenameUnsafeChars.ReplaceAllString(url, "")
}
