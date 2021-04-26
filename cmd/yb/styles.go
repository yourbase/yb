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
	"os"
	"strconv"

	"github.com/yourbase/commons/envvar"
)

// termStyles generates ANSI escape codes for specific styles.
// The zero value will not return any special escape codes.
// https://en.wikipedia.org/wiki/ANSI_escape_code
type termStyles bool

func termStylesFromEnv() termStyles {
	if os.Getenv("NO_COLOR") != "" {
		// https://no-color.org/
		return false
	}
	b, _ := strconv.ParseBool(envvar.Get("CLICOLOR", "1"))
	return termStyles(b)
}

// reset returns the escape code to change the output to default settings.
func (style termStyles) reset() string {
	if !style {
		return ""
	}
	return "\x1b[0m"
}

// target returns the escape code for formatting a target section heading.
func (style termStyles) target() string {
	if !style {
		return ""
	}
	// Bold
	return "\x1b[1m"
}

// command returns the escape code for formatting a command section heading.
func (style termStyles) command() string {
	return style.target()
}

// buildResult returns the escape code for formatting the final result message.
func (style termStyles) buildResult(success bool) string {
	switch {
	case !bool(style):
		return ""
	case !success:
		return style.failure()
	default:
		// Bold
		return "\x1b[1m"
	}
}

// failure returns the escape code for formatting error messages.
func (style termStyles) failure() string {
	if !style {
		return ""
	}
	// Red, but not bold to avoid using "bright red".
	return "\x1b[31m"
}

// debug returns the escape code for formatting debugging information.
func (style termStyles) debug() string {
	if !style {
		return ""
	}
	// Gray
	return "\x1b[90m"
}
