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

package main

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestLinePrefixWriter(t *testing.T) {
	start := time.Date(2021, time.April, 21, 14, 10, 23, 0, time.FixedZone("America/Los_Angeles", -7*60*60))
	const defaultPrefix = "default"
	const defaultPaddedPrefix = "default           |"
	tests := []struct {
		name   string
		prefix string
		writes []string
		want   string
	}{
		{
			name:   "Empty",
			prefix: defaultPrefix,
			writes: []string{""},
			want:   "",
		},
		{
			name:   "PartialLine",
			prefix: defaultPrefix,
			writes: []string{"foo"},
			want:   "14:10:23 " + defaultPaddedPrefix + " foo",
		},
		{
			name:   "FullLine",
			prefix: defaultPrefix,
			writes: []string{"foo\n"},
			want:   "14:10:23 " + defaultPaddedPrefix + " foo\n",
		},
		{
			name:   "MultipleLines/OneWrite",
			prefix: defaultPrefix,
			writes: []string{"foo\nbar\n"},
			want: "14:10:23 " + defaultPaddedPrefix + " foo\n" +
				"14:10:23 " + defaultPaddedPrefix + " bar\n",
		},
		{
			name:   "MultipleLines/MultipleWrites",
			prefix: defaultPrefix,
			writes: []string{"foo\n", "bar\n"},
			want: "14:10:23 " + defaultPaddedPrefix + " foo\n" +
				"14:10:24 " + defaultPaddedPrefix + " bar\n",
		},
		{
			name:   "MultipleLines/Split",
			prefix: defaultPrefix,
			writes: []string{"foo\nb", "ar\n"},
			want: "14:10:23 " + defaultPaddedPrefix + " foo\n" +
				"14:10:23 " + defaultPaddedPrefix + " bar\n",
		},
		{
			name:   "RSpecDots",
			prefix: defaultPrefix,
			writes: []string{".", ".", ".", ".", "\n"},
			want:   "14:10:23 " + defaultPaddedPrefix + " ....\n",
		},
		{
			name:   "LongPrefix",
			prefix: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			writes: []string{"foo\n"},
			want:   "14:10:23 xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx| foo\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			currTime := start
			out := new(strings.Builder)
			w := newLinePrefixWriter(out, test.prefix)
			w.now = func() time.Time {
				result := currTime
				currTime = currTime.Add(1 * time.Second)
				return result
			}
			for i, data := range test.writes {
				if n, err := io.WriteString(w, data); n != len(data) || err != nil {
					t.Errorf("Write[%d](%q) = %d, %v; want %d, <nil>", i, data, n, err, len(data))
				}
			}
			if diff := cmp.Diff(test.want, out.String()); diff != "" {
				t.Errorf("Output (-want +got):\n%s", diff)
			}
		})
	}
}
