#!/bin/bash

set -euo pipefail

source variables.sh

local_test_release="${TEST_RELEASE:-}"

# YourBase S3 artifacts Release
AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-}"
AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-}"
aws_disabled=true
[ -n "${AWS_ACCESS_KEY_ID}" -a -n "${AWS_SECRET_ACCESS_KEY}" ] && aws_disabled=false
if $aws_disabled; then
    echo "No AWS credentials, probably in Staging/Preview, giving up"
    # Non fatal, we won't get GH status errors when trying to release on Staging/Preview
    exit 0
fi

echo "Releasing ${CHANNEL} yb version ${VERSION} [${COMMIT}]..."

(
    for r in yb-*-*-${VERSION}.*; do
        for bucket in "yourbase-artifacts/yb/${VERSION}/" "yourbase-cats-bundles/"; do
            if [ -z "${local_test_release}" ]; then
                aws s3 cp "$r" "s3://$bucket"
            else
                echo "Local test, would run:"
                echo aws s3 cp $r s3://${bucket}
            fi
        done
    done
)

# Equinox install procedure
# TODO Migrate to GoRelease (for GH Releases and Homebrew tap)
#      or maybe use CATS to only send it to GH, and keep our home brew tap as is

APP="app_gtQEt1zkGMj"
PROJECT="yb"
TOKEN="${RELEASE_TOKEN:-}"
RELEASE_KEY="${RELEASE_KEY:-}"

cleanup() {
    rv=$?
    rm -rf "$tmpkeyfile"
    rm -rf equinox release-tool-stable-linux-amd64.tgz
    exit $rv
}

tmpkeyfile="$(mktemp)"
trap "cleanup" INT TERM EXIT

KEY_FILE="${tmpkeyfile}"

equinox_disabled=true

[ -n "${RELEASE_KEY}" -a -n "${TOKEN}" ] && equinox_disabled=false
if $equinox_disabled; then
    echo "No Equinox credentials, probably in Staging/Preview, giving up"
    # Non fatal, we won't get GH status errors when trying to release on Staging/Preview
    exit 0
fi

echo "${RELEASE_KEY}" > "${KEY_FILE}"

wget https://bin.equinox.io/c/mBWdkfai63v/release-tool-stable-linux-amd64.tgz
tar zxvf release-tool-stable-linux-amd64.tgz

if [ -n "${local_test_release}" ]; then
    echo -e "Local test, would run:
./equinox release
        --version=$VERSION
        --platforms='darwin_amd64 linux_amd64'
        --signing-key='${KEY_FILE}'
        --app='$APP'
        --token='${TOKEN}'
        --channel='${CHANNEL}'
    --
    -ldflags '-X main.version=$VERSION -X 'main.date=$(date -u '+%F-%T')' -X 'main.channel=$CHANNEL'${BUILD_COMMIT_INFO} -s -w'
    'github.com/yourbase/${PROJECT}'"

    exit 0
fi

./equinox release \
        --version=$VERSION \
        --platforms="darwin_amd64 linux_amd64" \
        --signing-key="${KEY_FILE}"  \
        --app="$APP" \
        --token="${TOKEN}" \
        --channel="${CHANNEL}" \
    -- \
    -ldflags "-X main.version=$VERSION -X 'main.date=$(date -u '+%F-%T')' -X 'main.channel=$CHANNEL'${BUILD_COMMIT_INFO} -s -w" \
    "github.com/yourbase/${PROJECT}"
