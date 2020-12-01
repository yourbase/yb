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

# release.sh determines the version from CI metadata then calls upon package.sh
# to create zip archives. The fleet distribution archive is sent to its bucket
# and the primary archive is set as a GitHub Action output.

set -euo pipefail

if [[ $# -gt 1 ]]; then
  echo "usage: release.sh [zip|debian|rpm]" 1>&2
  exit 64
fi

mode="${1:-zip}"

if [ -z "${VERSION:-}" ]; then
  tag_ref="${YB_GIT_BRANCH:-${GITHUB_REF:-}}"
  echo "Extracting version from tag ref $tag_ref" 1>&2
  VERSION="$(echo "$tag_ref" | sed -e 's|^refs/tags/||')"
  export VERSION
fi

if [ -z "$VERSION" ]; then
  echo "No version provided, won't release" 1>&2
  exit 1
fi

if echo "$VERSION" | grep -vqo '^v'; then
  echo "Doesn't start with a \"v\" when it should, not releasing" 1>&2
  exit 1
fi

if echo "$VERSION" | grep -q -- '-[-.a-zA-Z0-9]\+'; then
  # Pre-release
  export CHANNEL="preview"
else
  export CHANNEL="stable"
fi

# dryrunnable echoes a command to stderr, but doesn't run it if
# TEST_RELEASE is set.
dryrunnable() {
  echo "# $*" 1>&2
  if [[ -z "${TEST_RELEASE:-}" ]]; then
    "$@"
  fi
}

srcroot="$(dirname "$(dirname "${BASH_SOURCE[0]}")" )"
case "$mode" in
  zip)
    triple="$( "$srcroot/release/package.sh" )"
    dryrunnable aws s3 cp "yb_${triple}_cats.zip" "s3://yourbase-cats-bundles/yb_${triple}.zip"
    dryrunnable aws s3 cp "yb-next_${triple}_cats.zip" "s3://yourbase-cats-bundles/yb-next_${triple}.zip"
    echo "::set-output name=file::yb_${triple}.zip"
    ;;
  debian)
    debfile="$( "$srcroot/release/debpackage.sh" )"
    dryrunnable aptblob upload \
      -k 86669479255976751E2352D6649960422CC7F0F8 \
      s3://yourbase-apt-repo \
      preview \
      "$debfile"
    if [[ "$CHANNEL" == stable ]]; then
      dryrunnable aptblob upload \
        -k 86669479255976751E2352D6649960422CC7F0F8 \
        s3://yourbase-apt-repo \
        stable \
        "$debfile"
    fi
    echo "::set-output name=file::${debfile}"
    ;;
  rpm)
    rpmfile="$( "$srcroot/release/rpmpackage.sh" )"
    echo "::set-output name=name::$( basename "$rpmfile" )"
    echo "::set-output name=file::${rpmfile}"
    ;;
  *)
    echo "usage: release.sh [debian|zip|rpm]" 1>&2
    exit 1
    ;;
esac
