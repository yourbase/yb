#!/bin/bash
# Copyright 2020 YourBase Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# SPDX-License-Identifier: Apache-2.0

# package.sh builds two zip archives:
# 1. The minimal archive sent to our fleet release system, suffixed "_cats.zip".
# 2. The full archive to be consumed by end-users, suffixed ".zip".
#
# Upon success, it prints the base name of the archives to stdout.

set -euo pipefail

if [[ $# -ne 0 ]]; then
  echo "usage: VERSION=vX.Y.Z package.sh" 1>&2
  exit 64
fi

# Identify directories and create temporary staging directory.
outdir="$( pwd )"
srcroot="$(dirname "${BASH_SOURCE[0]}")"
stageroot="$(mktemp -d 2>/dev/null || mktemp -d -t yb_release)"
cleanup() {
  rm -rf "$stageroot"
}
trap cleanup EXIT

# Compute bundle name.
VERSION="${VERSION:-DEVELOPMENT}"
GOOS="$( go env GOOS )"
export GOOS
GOARCH="$( go env GOARCH )"
export GOARCH
bundle="yb_${VERSION}_${GOOS}_${GOARCH}"
distroot="$stageroot/$bundle"

# mkzip creates a zip archive of $distroot in the current directory with the
# name given as the first argument.
mkzip() {
  rm -f "$1" 1>&2
  ( cd "$stageroot" && zip -r "$outdir/$1" "$bundle" ) 1>&2
}

# First: create minimal fleet distribution.
./build.sh "$distroot/yb"
mkzip "${bundle}_cats.zip"

# Next: create end-user-friendly distribution.
cp "$srcroot/README.md" "$srcroot/LICENSE" "$distroot/"
mkzip "${bundle}.zip"

# Output base name of bundle.
echo "$bundle"
