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
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/google/shlex"
	"github.com/spf13/cobra"
	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/biome/remote"
	"github.com/yourbase/yb/internal/build"
	"github.com/yourbase/yb/internal/ybdata"
	"zombiezen.com/go/log"
)

type remoteShellCmd struct {
	baseURL    *url.URL
	target     string
	netrcFiles []string
	env        []commandLineEnv
}

func newRemoteShellCmd() *cobra.Command {
	cmd := new(remoteShellCmd)
	c := &cobra.Command{
		Use:   "remote-shell [flags] URL",
		Short: "Interactive remote shell",
		Args:  cobra.ExactArgs(1),
		RunE: func(cc *cobra.Command, args []string) error {
			var err error
			cmd.baseURL, err = url.Parse(args[0])
			if err != nil {
				return err
			}
			return cmd.run(cc.Context())
		},
		DisableFlagsInUseLine: true,
		Hidden:                true,
	}
	c.Flags().StringVarP(&cmd.target, "target", "t", "default", "target to build")
	netrcFlagVar(c.Flags(), &cmd.netrcFiles)
	envFlagsVar(c.Flags(), &cmd.env)
	return c
}

func (cmd *remoteShellCmd) run(ctx context.Context) error {
	dataDirs, err := ybdata.DirsFromEnv()
	if err != nil {
		return err
	}
	downloader := ybdata.NewDownloader(dataDirs.Downloads())
	baseEnv, err := envFromCommandLine(cmd.env)
	if err != nil {
		return err
	}

	pkg, subdir, err := findPackage()
	if err != nil {
		return err
	}
	execTarget := pkg.Targets[cmd.target]
	if execTarget == nil {
		return fmt.Errorf("%s: no such target", cmd.target)
	}
	targets := yb.BuildOrder(execTarget)

	// Build dependencies before running command.
	err = doTargetList(ctx, pkg, targets[:len(targets)-1], &doOptions{
		dataDirs:   dataDirs,
		downloader: downloader,
		baseEnv:    baseEnv,
		netrcFiles: cmd.netrcFiles,
	})
	if err != nil {
		return err
	}
	log.Infof(ctx, "Connecting to %v ...", cmd.baseURL)
	bio, err := remote.Connect(ctx, http.DefaultClient, cmd.baseURL, pkg.Path)
	if err != nil {
		return err
	}
	log.Infof(ctx, "Connected!")
	sys := build.Sys{
		Biome:      bio,
		Downloader: downloader,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}
	execBiome, err := build.Setup(ctx, sys, execTarget)
	if err != nil {
		return err
	}
	defer func() {
		if err := execBiome.Close(); err != nil {
			log.Warnf(ctx, "Clean up environment: %v", err)
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		os.Stdout.WriteString("> ")
		if !scanner.Scan() {
			break
		}
		argv, err := shlex.Split(scanner.Text())
		if err != nil {
			fmt.Fprintln(os.Stderr, "parse:", err)
			continue
		}
		if len(argv) == 0 {
			continue
		}
		err = execBiome.Run(ctx, &biome.Invocation{
			Argv:   argv,
			Dir:    subdir,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	return nil
}
