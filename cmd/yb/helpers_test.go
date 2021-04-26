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
	"testing"

	"github.com/yourbase/yb"
)

func TestWillUseDocker(t *testing.T) {
	tests := []struct {
		mode    executionMode
		targets []*yb.Target
		want    bool
	}{
		{
			mode: noContainer,
			targets: []*yb.Target{
				{Name: "default", UseContainer: false},
				{Name: "foo", UseContainer: false},
			},
			want: false,
		},
		{
			mode: preferHost,
			targets: []*yb.Target{
				{Name: "default", UseContainer: false},
				{Name: "foo", UseContainer: false},
			},
			want: false,
		},
		{
			mode: useContainer,
			targets: []*yb.Target{
				{Name: "default", UseContainer: false},
				{Name: "foo", UseContainer: false},
			},
			want: true,
		},
		{
			mode: noContainer,
			targets: []*yb.Target{
				{Name: "default", UseContainer: true},
				{Name: "foo", UseContainer: false},
			},
			want: true,
		},
		{
			mode: preferHost,
			targets: []*yb.Target{
				{Name: "default", UseContainer: true},
				{Name: "foo", UseContainer: false},
			},
			want: true,
		},
		{
			mode: useContainer,
			targets: []*yb.Target{
				{Name: "default", UseContainer: true},
				{Name: "foo", UseContainer: false},
			},
			want: true,
		},
	}
	for _, test := range tests {
		got := willUseDocker(test.mode, test.targets)
		if got != test.want {
			t.Errorf("willUseDocker(%d, %+v) = %t; want %t", test.mode, test.targets, got, test.want)
		}
	}
}
