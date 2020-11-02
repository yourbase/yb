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
	"io"

	"zombiezen.com/go/log"
)

// BiomeCloser is the interface for biomes that have resources that need to
// be cleaned up after use.
//
// The behavior of any biome methods called after or concurrently with Close
// is undefined.
type BiomeCloser interface {
	Biome
	io.Closer
}

// NopCloser returns a BiomeCloser with a no-op Close method wrapping the
// provided biome.
func NopCloser(bio Biome) BiomeCloser {
	return nopCloser{bio}
}

type nopCloser struct {
	Biome
}

func (n nopCloser) Close() error {
	return nil
}

func (n nopCloser) WriteFile(ctx context.Context, path string, src io.Reader) error {
	return forwardWriteFile(ctx, n.Biome, path, src)
}

func (n nopCloser) MkdirAll(ctx context.Context, path string) error {
	return forwardMkdirAll(ctx, n.Biome, path)
}

func (n nopCloser) EvalSymlinks(ctx context.Context, path string) (string, error) {
	return forwardEvalSymlinks(ctx, n.Biome, path)
}

// WithClose returns a new biome that wraps another biome to call the given
// function at the beginning of Close, before the underlying biome's Close
// method is called. If the function returns an error, it will be returned from
// Close, but the underlying biome's Close method will still be called.
func WithClose(bio BiomeCloser, closeFunc func() error) BiomeCloser {
	if bio == nil {
		panic("biome.WithClose called with nil biome")
	}
	if closeFunc == nil {
		panic("biome.WithClose called with nil function")
	}
	return closer{bio, closeFunc}
}

type closer struct {
	BiomeCloser
	closeFunc func() error
}

func (c closer) Close() error {
	funcErr := c.closeFunc()
	biomeErr := c.BiomeCloser.Close()
	if funcErr != nil {
		if biomeErr != nil {
			log.Errorf(context.Background(), "Cleaning up environment: %v", biomeErr)
		}
		return funcErr
	}
	return biomeErr
}

func (c closer) WriteFile(ctx context.Context, path string, src io.Reader) error {
	return forwardWriteFile(ctx, c.BiomeCloser, path, src)
}

func (c closer) MkdirAll(ctx context.Context, path string) error {
	return forwardMkdirAll(ctx, c.BiomeCloser, path)
}

func (c closer) EvalSymlinks(ctx context.Context, path string) (string, error) {
	return forwardEvalSymlinks(ctx, c.BiomeCloser, path)
}
