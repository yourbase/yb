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

# package.sh builds three zip archives:
# 1. The minimal archive sent to our fleet release system. This will be named
#    something like "yb_v1.2.3_linux_amd64_cats.zip".
# 2. A second minimal archive sent to our fleet release system with the
#    executable name "yb-next". This will be named something like
#    "yb-next_v1.2.3_linux_amd64_cats.zip".
# 3. The full archive to be consumed by end-users. This will be named something
#    like "yb_v1.2.3_linux_amd64.zip".
#
# Upon success, it prints the VERSION/GOOS/GOARCH triple to stdout. This will
# be something like "v1.2.3_linux_amd64".

set -euo pipefail

if [[ $# -ne 0 ]]; then
  echo "usage: VERSION=vX.Y.Z package.sh" 1>&2
  exit 64
fi

# Identify directories and create temporary staging directory.
outdir="$( pwd )"
srcroot="$(dirname "$(dirname "${BASH_SOURCE[0]}")" )"
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
triple="${VERSION}_${GOOS}_${GOARCH}"
distname="yb_${triple}"
distroot="$stageroot/$distname"

# mkzip creates a zip archive of $distroot in the current directory with the
# name given as the first argument.
mkzip() {
  rm -f "$1" 1>&2
  ( cd "$stageroot" && zip -r "$outdir/$1" "$distname" ) 1>&2
}

# First: create minimal fleet distributions.
"$srcroot/release/build.sh" "$distroot/yb-next"
mkzip "yb-next_${triple}_cats.zip"

mv "$distroot/yb-next" "$distroot/yb"
mkzip "yb_${triple}_cats.zip"

# Next: create end-user-friendly distribution.
cp "$srcroot/README.md" "$srcroot/LICENSE" "$srcroot/CHANGELOG.md" "$distroot/"
mkzip "yb_${triple}.zip"

# Output triple.
echo "$triple"
