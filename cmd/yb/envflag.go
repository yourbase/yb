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

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/yourbase/commons/ini"
	"github.com/yourbase/yb/internal/biome"
)

const (
	envLiteral = false
	envFile    = true
)

type commandLineEnv struct {
	envType bool
	key     string
	value   string
}

// envFromCommandLine builds a biome environment from command line flag
// arguments, reading .env files as requested.
func envFromCommandLine(args []commandLineEnv) (biome.Environment, error) {
	env := biome.Environment{Vars: make(map[string]string)}
	for _, a := range args {
		switch a.envType {
		case envLiteral:
			env.Vars[a.key] = a.value
		case envFile:
			f, err := os.Open(a.value)
			if err != nil {
				return biome.Environment{}, fmt.Errorf("load env file: %w", err)
			}
			parsed, err := ini.Parse(f, nil)
			f.Close() // Any errors from here are not actionable.
			if err != nil {
				return biome.Environment{}, fmt.Errorf("load env file: %s: %w", a.value, err)
			}
			if parsed.HasSections() {
				return biome.Environment{}, fmt.Errorf("load env file: %s: sections not permitted", a.value)
			}
			for k, values := range parsed.Section("") {
				env.Vars[k] = values[len(values)-1]
			}
		default:
			panic("unreachable")
		}
	}
	return env, nil
}

// envFlagsVar registers the --env and --env-file flags.
func envFlagsVar(flags *pflag.FlagSet, env *[]commandLineEnv) {
	flags.VarP(envLiteralFlag{env}, "env", "e", "Set an environment variable (can be passed multiple times)")
	flags.VarP(envFileFlag{env}, "env-file", "E", "Load environment variables from a .env file (can be passed multiple times)")
}

type envLiteralFlag struct {
	env *[]commandLineEnv
}

func (f envLiteralFlag) String() string {
	var parts []string
	for _, e := range *f.env {
		if e.envType == envLiteral {
			parts = append(parts, e.key+"="+e.value)
		}
	}
	return strings.Join(parts, " ")
}

func (f envLiteralFlag) Set(arg string) error {
	i := strings.IndexByte(arg, '=')
	if i == -1 {
		return fmt.Errorf("invalid environment variable %q: missing '='", arg)
	}
	if i == 0 {
		return fmt.Errorf("invalid environment variable %q: missing variable name", arg)
	}
	*f.env = append(*f.env, commandLineEnv{
		envType: envLiteral,
		key:     arg[:i],
		value:   arg[i+1:],
	})
	return nil
}

func (f envLiteralFlag) Type() string {
	return "key=value"
}

type envFileFlag struct {
	env *[]commandLineEnv
}

func (f envFileFlag) String() string {
	var parts []string
	for _, e := range *f.env {
		if e.envType == envFile {
			parts = append(parts, e.key+"="+e.value)
		}
	}
	return strings.Join(parts, string(os.PathListSeparator))
}

func (f envFileFlag) Set(arg string) error {
	*f.env = append(*f.env, commandLineEnv{
		envType: envFile,
		value:   arg,
	})
	return nil
}

func (f envFileFlag) Type() string {
	return "path"
}
