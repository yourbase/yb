// Copyright 2019 The Go Cloud Development Kit Authors
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

// This package came from https://github.com/google/go-cloud/blob/c3d5b8353de0f7cc033276f04b4255d0ad10140f/internal/cmd/gocdk/internal/docker/docker.go

package imageref

import (
	"strings"
)

// Parse parses a Docker image reference, as documented in
// https://godoc.org/github.com/docker/distribution/reference. It permits some
// looseness in characters, and in particular, permits the empty name form
// ":foo". It is guaranteed that name + tag + digest == s.
func Parse(s string) (name, tag, digest string) {
	if i := strings.LastIndexByte(s, '@'); i != -1 {
		s, digest = s[:i], s[i:]
	}
	i := strings.LastIndexFunc(s, func(c rune) bool { return !isTagChar(c) })
	if i == -1 || s[i] != ':' {
		return s, "", digest
	}
	return s[:i], s[i:], digest
}

// Registry parses the registry (everything before the first slash) from
// a Docker image reference or name.
func Registry(s string) string {
	name, _, _ := Parse(s)
	i := strings.IndexByte(name, '/')
	const defaultRegistry = "registry-1.docker.io"
	if i == -1 {
		return defaultRegistry
	}
	registry := name[:i]
	if !strings.ContainsAny(registry, ".:") {
		return defaultRegistry
	}
	return registry
}

func isTagChar(c rune) bool {
	return 'a' <= c && c <= 'z' ||
		'A' <= c && c <= 'Z' ||
		'0' <= c && c <= '9' ||
		c == '_' || c == '-' || c == '.'
}
