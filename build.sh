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

# build.sh builds a release binary with build information stamped in.

set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: build.sh OUT" 1>&2
  exit 64
fi

VERSION="${VERSION:-DEVELOPMENT}"
CHANNEL="${CHANNEL:-development}"
COMMIT="${YB_GIT_COMMIT:-${GITHUB_SHA:-}}"
if [ -z "$COMMIT" ]; then
  COMMIT="$(git rev-parse HEAD)" || COMMIT=''
fi

echo "Building yb release binary..." 1>&2
echo "VERSION=$VERSION" 1>&2
echo "CHANNEL=$CHANNEL" 1>&2
echo "COMMIT=$COMMIT" 1>&2

go build \
  -buildmode=pie \
  -ldflags "-X 'main.version=$VERSION' -X 'main.date=$(date -u '+%FT%TZ')' -X 'main.channel=$CHANNEL' -X 'main.commitSHA=$COMMIT' -s -w" \
  -o "$1" \
  github.com/yourbase/yb 1>&2
