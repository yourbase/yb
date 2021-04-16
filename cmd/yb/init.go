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
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourbase/yb"
	"zombiezen.com/go/log"
)

type initCmd struct {
	dir       string
	language  string
	outPath   string
	overwrite bool
	quiet     bool
}

func newInitCmd() *cobra.Command {
	cmd := new(initCmd)
	c := &cobra.Command{
		Use:   "init [flags] [DIR]",
		Short: "Initialize directory",
		Long:  "Initialize package directory with a " + yb.PackageConfigFilename + " file.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cc *cobra.Command, args []string) error {
			if len(args) == 1 {
				cmd.dir = args[0]
			}
			return cmd.run(cc.Context())
		},
		DisableFlagsInUseLine: true,
	}
	c.Flags().StringVar(&cmd.language, "lang", langDetectFlagValue, "Programming language to create for")
	c.Flags().StringVarP(&cmd.outPath, "output", "o", "", "Output file (- for stdout)")
	c.Flags().BoolVarP(&cmd.overwrite, "force", "f", false, "Overwrite existing file")
	c.Flags().BoolVarP(&cmd.quiet, "quiet", "q", false, "Suppress non-error output")
	return c
}

func (cmd *initCmd) run(ctx context.Context) (cmdErr error) {
	const stdoutToken = "-"

	if !cmd.quiet {
		log.Infof(ctx, "Welcome to YourBase!")
	}

	// Find or create target directory.
	dir, err := filepath.Abs(cmd.dir)
	if err != nil {
		return err
	}
	log.Debugf(ctx, "Directory: %s", dir)
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return err
	}

	// Do a quick check to see if Docker works.
	client, err := connectDockerClient(true)
	if err != nil {
		return err
	}
	log.Debugf(ctx, "Checking Docker connection...")
	pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
	err = client.PingWithContext(pingCtx)
	cancelPing()
	if err != nil {
		log.Warnf(ctx, "yb can't connect to Docker. You might encounter issues during build. Error: %v", err)
	} else if !cmd.quiet {
		log.Infof(ctx, "yb connected to Docker successfully!")
	}

	// Find the appropriate template.
	language := cmd.language
	if language == langDetectFlagValue {
		if !cmd.quiet {
			log.Infof(ctx, "Detecting your programming language...")
		}
		language, err = detectLanguage(ctx, dir)
		if err != nil {
			return err
		}
		if language == "" {
			log.Warnf(ctx, "Unable to detect language; generating a generic build configuration.")
		} else if !cmd.quiet {
			log.Infof(ctx, "Found %s!", language)
		}
	}
	templateData, err := packageConfigTemplate(language)
	if err != nil {
		return err
	}

	// Write template to requested file.
	var out io.Writer = os.Stdout
	outPath := cmd.outPath
	if outPath != stdoutToken {
		if outPath == "" {
			outPath = filepath.Join(dir, yb.PackageConfigFilename)
		}
		if !cmd.quiet {
			log.Infof(ctx, "Writing package configuration to %s", outPath)
		}
		flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		if !cmd.overwrite {
			flags |= os.O_EXCL
		}
		f, err := os.OpenFile(outPath, flags, 0o666)
		if err != nil {
			return err
		}
		out = f
		defer func() {
			if closeErr := f.Close(); closeErr != nil {
				if cmdErr == nil {
					cmdErr = closeErr
				} else {
					log.Warnf(ctx, "%v", closeErr)
				}
			}
		}()
	}
	if _, err := io.WriteString(out, templateData); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	if !cmd.quiet && filepath.Base(outPath) == yb.PackageConfigFilename {
		log.Infof(ctx, "All done! Try running `yb build` to build your project.")
		log.Infof(ctx, "Edit %s to configure your build process.", outPath)
	}
	return nil
}

const (
	langGenericFlagValue = ""
	langDetectFlagValue  = "auto"

	langPythonFlagValue = "python"
	langRubyFlagValue   = "ruby"
	langGoFlagValue     = "go"
)

func detectLanguage(ctx context.Context, dir string) (string, error) {
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("detect project language: %w", err)
	}
	detected := langGenericFlagValue
	for _, info := range infos {
		prevDetected := detected
		switch name := info.Name(); {
		case name == "go.mod" || strings.HasSuffix(name, ".go"):
			detected = langGoFlagValue
		case name == "requirements.txt" || strings.HasSuffix(name, ".py"):
			detected = langPythonFlagValue
		case name == "Gemfile" || strings.HasSuffix(name, ".rb"):
			detected = langRubyFlagValue
		default:
			continue
		}
		if prevDetected != langGenericFlagValue && detected != prevDetected {
			log.Debugf(ctx, "Detected both %s and %s; returning generic", detected, prevDetected)
			return langGenericFlagValue, nil
		}
	}
	return detected, nil
}

//go:embed init_templates/*.yml
var packageConfigTemplateFiles embed.FS

func packageConfigTemplate(name string) (string, error) {
	path := "init_templates/" + name + ".yml"
	if name == langGenericFlagValue {
		path = "init_templates/generic.yml"
	}
	f, err := packageConfigTemplateFiles.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("unknown language %q", name)
	}
	if err != nil {
		return "", fmt.Errorf("load template for language %q: %w", name, err)
	}
	defer f.Close()
	out := new(strings.Builder)
	if _, err := io.Copy(out, f); err != nil {
		return "", fmt.Errorf("load template for language %q: %w", name, err)
	}
	return out.String(), nil
}
