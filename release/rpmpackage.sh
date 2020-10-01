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

# rpmpackage.sh builds a RPM binary package from the Go binary.

set -euo pipefail

if [[ $# -ne 0 ]]; then
  echo "usage: VERSION=vX.Y.Z rpmpackage.sh" 1>&2
  exit 64
fi

# Identify directories and create temporary staging directory.
outdir="$( pwd )"
srcroot="$(dirname "$(dirname "${BASH_SOURCE[0]}")" )"

# Compute version.
VERSION="${VERSION:-DEVELOPMENT}"
rpmversion="${VERSION/#v/}"
rpmversion="${rpmversion//-/\~}"

# Verify that OS/architecture is something we handle.
if [[ "$( go env GOOS )" != linux ]]; then
  echo "Not running on Linux; quitting." 1>&2
  exit 1
fi
case "$( go env GOARCH )" in
  amd64)
    rpmarch="x86_64"
    ;;
  *)
    echo "TODO: Unknown architecture. GOARCH=$( go env GOARCH )" 1>&2
    exit 1
    ;;
esac

# Create binary package.
( cd "$srcroot" && rpmbuild \
  --build-in-place \
  --define="version $rpmversion" \
  --define="_rpmdir $outdir" \
  -bb release/yb.spec ) 1>&2
echo "${rpmarch}/yb-${rpmversion}-1.${rpmarch}.rpm"

