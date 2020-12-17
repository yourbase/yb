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

// TODO(light): In Go 1.16, embed files rather than trying to write this
// as Go string literals.

const packageConfigTemplateHeader = "# YourBase build configuration\n" +
	"# See https://docs.yourbase.io/ for reference.\n\n"

var packageConfigTemplates = map[string]string{
	langGenericFlagValue: packageConfigTemplateHeader +
		"# We couldn't determine what programming language your project uses,\n" +
		"# so we generated a generic build configuration.\n" +
		"\n" +
		"build_targets:\n" +
		"  - name: default\n" +
		"    # Change 'make' to run the command you want.\n" +
		"    # You can also add more commands to be run in the list.\n" +
		"    commands:\n" +
		"      - make\n" +
		ciBuildTemplateStanza,

	langPythonFlagValue: packageConfigTemplateHeader +
		"dependencies:\n" +
		"  build:\n" +
		"    - python:3.6.3\n" +
		"\n" +
		"build_targets:\n" +
		"  - name: default\n" +
		"    commands:\n" +
		"      - pip install -r requirements.txt\n" +
		"      - python tests.py\n" +
		ciBuildTemplateStanza,

	langGoFlagValue: packageConfigTemplateHeader +
		"dependencies:\n" +
		"  build:\n" +
		"    - go:1.15.6\n" +
		"\n" +
		"build_targets:\n" +
		"  - name: default\n" +
		"    commands:\n" +
		"      - go test ./...\n" +
		ciBuildTemplateStanza,

	langRubyFlagValue: packageConfigTemplateHeader +
		"dependencies:\n" +
		"  build:\n" +
		"    - ruby:2.6.3\n" +
		"\n" +
		"build_targets:\n" +
		"  - name: default\n" +
		"    commands:\n" +
		"      - gem install bundler\n" +
		"      - bundle install\n" +
		"      - bundle exec rspec\n" +
		ciBuildTemplateStanza,
}

const ciBuildTemplateStanza = `
# This section configures which targets get built on CI.
ci:
  builds:
    - name: default
      build_target: default
      # If you only want certain events, uncomment the following line.
      # when: branch IS 'main' OR action IS 'pull_request'
`
