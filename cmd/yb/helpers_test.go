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
	"fmt"
	"strings"
	"testing"

	"github.com/yourbase/yb"
)

func TestWillUseDocker(t *testing.T) {
	networkAvailable, _ := hostHasDockerNetwork()
	tests := []struct {
		mode        executionMode
		targets     []*yb.Target
		want        bool
		forCommands bool
	}{
		{
			mode: noContainer,
			targets: []*yb.Target{
				{Name: "default", UseContainer: false},
				{Name: "foo", UseContainer: false},
			},
			want:        false,
			forCommands: false,
		},
		{
			mode: preferHost,
			targets: []*yb.Target{
				{Name: "default", UseContainer: false},
				{Name: "foo", UseContainer: false},
			},
			want:        false,
			forCommands: false,
		},
		{
			mode: useContainer,
			targets: []*yb.Target{
				{Name: "default", UseContainer: false},
				{Name: "foo", UseContainer: false},
			},
			want:        true,
			forCommands: true,
		},
		{
			mode: noContainer,
			targets: []*yb.Target{
				{Name: "default", UseContainer: true},
				{Name: "foo", UseContainer: false},
			},
			want:        true,
			forCommands: true,
		},
		{
			mode: preferHost,
			targets: []*yb.Target{
				{Name: "default", UseContainer: true},
				{Name: "foo", UseContainer: false},
			},
			want:        true,
			forCommands: true,
		},
		{
			mode: useContainer,
			targets: []*yb.Target{
				{Name: "default", UseContainer: true},
				{Name: "foo", UseContainer: false},
			},
			want:        true,
			forCommands: true,
		},
		{
			mode: noContainer,
			targets: []*yb.Target{
				{Name: "default", UseContainer: false, Resources: map[string]*yb.ResourceDefinition{"foo": {}}},
				{Name: "foo", UseContainer: false},
			},
			want:        true,
			forCommands: !networkAvailable,
		},
		{
			mode: preferHost,
			targets: []*yb.Target{
				{Name: "default", UseContainer: false, Resources: map[string]*yb.ResourceDefinition{"foo": {}}},
				{Name: "foo", UseContainer: false},
			},
			want:        true,
			forCommands: !networkAvailable,
		},
		{
			mode: useContainer,
			targets: []*yb.Target{
				{Name: "default", UseContainer: false, Resources: map[string]*yb.ResourceDefinition{"foo": {}}},
				{Name: "foo", UseContainer: false},
			},
			want:        true,
			forCommands: true,
		},
	}

	formatTargets := func(targets []*yb.Target) string {
		stringList := make([]string, 0, len(targets))
		for _, tgt := range targets {
			stringList = append(stringList, fmt.Sprintf("{Name:%q UseContainer:%t Resources:%+v}", tgt.Name, tgt.UseContainer, tgt.Resources))
		}
		return "[" + strings.Join(stringList, " ") + "]"
	}

	for _, test := range tests {
		if got := willUseDocker(test.mode, test.targets); got != test.want {
			t.Errorf("willUseDocker(%d, %s) = %t; want %t", test.mode, formatTargets(test.targets), got, test.want)
		}
		if got := willUseDockerForCommands(test.mode, test.targets); got != test.forCommands {
			t.Errorf("willUseDockerForCommands(%d, %s) = %t; want %t", test.mode, formatTargets(test.targets), got, test.forCommands)
		}
	}
}
