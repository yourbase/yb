// Copyright 2020 YourBase Inc.
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
	"os"

	"github.com/spf13/cobra"
)

func newGenCompleteCmd() *cobra.Command {
	c := &cobra.Command{
		Use:                   "gen-complete [flags] SHELL",
		Short:                 "Generate shell completions (dev only)",
		Args:                  cobra.ExactArgs(1),
		Hidden:                true,
		DisableFlagsInUseLine: true,
	}
	outputPath := c.Flags().StringP("output", "o", "-", "Output file ('-' for stdout)")
	c.RunE = func(cc *cobra.Command, args []string) (err error) {
		out := os.Stdout
		if *outputPath != "-" {
			f, err := os.Create(*outputPath)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := f.Close(); err == nil && closeErr != nil {
					err = closeErr
				}
			}()
			out = f
		}

		switch args[0] {
		case "bash":
			return cc.Root().GenBashCompletion(out)
		case "zsh":
			return cc.Root().GenZshCompletion(out)
		case "fish":
			return cc.Root().GenFishCompletion(out, true)
		case "powershell":
			return cc.Root().GenPowerShellCompletion(out)
		default:
			return fmt.Errorf("unknown shell %q", args[0])
		}
	}
	return c
}
