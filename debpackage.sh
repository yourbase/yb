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

# debpackage.sh builds a Debian binary package from the Go binary.
# See https://tldp.org/HOWTO/html_single/Debian-Binary-Package-Building-HOWTO/

set -euo pipefail

if [[ $# -ne 0 ]]; then
  echo "usage: VERSION=vX.Y.Z debpackage.sh" 1>&2
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

# Compute version.
VERSION="${VERSION:-DEVELOPMENT}"
debversion="${VERSION/#v/}"
debversion="${debversion//-/\~}"

# Verify that OS/architecture is something we handle.
if [[ "$( go env GOOS )" != linux ]]; then
  echo "Not running on Linux; quitting." 1>&2
  exit 1
fi
case "$( go env GOARCH )" in
  amd64)
    debarch="amd64"
    ;;
  *)
    echo "TODO: Unknown architecture. GOARCH=$( go env GOARCH )" 1>&2
    exit 1
    ;;
esac

# Stage Debian tree.
mkdir -m 755 \
  "$stageroot/DEBIAN" \
  "$stageroot/usr" \
  "$stageroot/usr/bin" \
  "$stageroot/usr/share" \
  "$stageroot/usr/share/doc" \
  "$stageroot/usr/share/doc/yb"
install -m 644 "$srcroot/debian/control" "$stageroot/DEBIAN/control"
sed -i -e "s/^Version:.*/Version: $debversion/" "$stageroot/DEBIAN/control"
sed -i -e "s/^Architecture: any\$/Architecture: $debarch/" "$stageroot/DEBIAN/control"
install -m 644 "$srcroot/debian/copyright" "$stageroot/usr/share/doc/yb/copyright"
"$srcroot/build.sh" "$stageroot/usr/bin/yb"
chmod 755 "$stageroot/usr/bin/yb"

mkdir -m 755 \
  "$stageroot/usr/share/lintian" \
  "$stageroot/usr/share/lintian/overrides"
install -m 644 \
  "$srcroot/debian/yb.lintian-overrides" \
  "$stageroot/usr/share/lintian/overrides/yb"

# Create binary package.
fakeroot dpkg-deb --build "$stageroot" "$outdir" 1>&2
debfile="yb_${debversion}_${debarch}.deb"
lintian "$debfile" 1>&2
echo "$debfile"

