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
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/yourbase/yb/internal/ybdata"
	"zombiezen.com/go/log"
)

type cleanCmd struct {
	targets []string
}

func newCleanCmd() *cobra.Command {
	cmd := new(cleanCmd)
	c := &cobra.Command{
		Use:   "clean [flags] [TARGET [...]]",
		Short: "Delete build cache",
		Long: "clean deletes the build cache available as $HOME in the build\n" +
			"environment. If no arguments are given, all targets' caches\n" +
			"for the current package will be deleted. Otherwise, only the\n" +
			"caches for the given targets will be deleted.",
		RunE: func(cc *cobra.Command, args []string) error {
			cmd.targets = args
			return cmd.run(cc.Context())
		},
		DisableFlagsInUseLine: true,
		ValidArgsFunction: func(cc *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return autocompleteTargetName(toComplete)
		},
	}
	return c
}

func (cmd *cleanCmd) run(ctx context.Context) error {
	dirs, err := ybdata.DirsFromEnv()
	if err != nil {
		return err
	}
	pkg, _, err := findPackage()
	if err != nil {
		return err
	}

	if len(cmd.targets) == 0 {
		// Delete all caches for package.
		dir := dirs.BuildHomeRoot(pkg.Path)
		log.Debugf(ctx, "Deleting %s", dir)
		return os.RemoveAll(dir)
	}

	ok := true
	for _, tgt := range cmd.targets {
		dir := filepath.Join(dirs.BuildHomeRoot(pkg.Path), tgt)
		log.Debugf(ctx, "Deleting %s", dir)
		if err := os.RemoveAll(dir); err != nil {
			log.Errorf(ctx, "Failed to remove directory: %v", err)
			ok = false
		}
	}
	if !ok {
		return errors.New("failed to clean directories")
	}
	return nil
}
