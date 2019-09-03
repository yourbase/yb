#!/bin/bash

set -eux

APP="app_gtQEt1zkGMj"
PROJECT="yb"
VERSION="$(echo $YB_GIT_BRANCH | sed -e 's|refs/tags/||g')"
TOKEN="${RELEASE_TOKEN}"
RELEASE_KEY="${RELEASE_KEY}"

if [ -z "${CHANNEL}" ]; then
  echo "Channel not set, will release as unstable"
  CHANNEL="unstable"
fi

if [ "${CHANNEL}" == "development" ]; then 
  echo "Channel is development, setting version to timestamp"
  VERSION="$(date +"%Y%m%d%H%M%S")"
fi

if [ "${CHANNEL}" == "unstable" ]; then 
  echo "Channel is unstable, setting version to timestamp"
  VERSION="$(date +"%Y%m%d%H%M%S")"
fi


umask 077

cleanup() {
    rv=$?
    rm -rf "$tmpdir"
    exit $rv
}

tmpdir="$(mktemp)"
trap "cleanup" INT TERM EXIT

KEY_FILE="${tmpdir}"
echo "${RELEASE_KEY}" > "${KEY_FILE}"

wget https://bin.equinox.io/c/mBWdkfai63v/release-tool-stable-linux-amd64.tgz
tar zxvf release-tool-stable-linux-amd64.tgz

./equinox release \
        --version=$VERSION \
        --platforms="darwin_amd64 linux_amd64" \
        --signing-key="${KEY_FILE}"  \
        --app="$APP" \
        --token="${TOKEN}" \
        --channel="${CHANNEL}" \
	-- \
	-ldflags "-X main.version=$VERSION -X 'main.date=$(date)'" \
	"github.com/yourbase/${PROJECT}"
