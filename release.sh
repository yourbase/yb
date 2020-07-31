#!/bin/bash

set -euo pipefail

VERSION="${1:-$(echo "${YB_GIT_BRANCH:-}" | sed -e 's:^refs/tags/::')}"
if [[ -z "$VERSION" ]]; then
  echo "release.sh: no version provided" 1>&2
  exit 1
fi

if $(echo $VERSION | grep -vqo '^v'); then
    echo "Doesn't start with a \"v\" when it should, not releasing"
    exit 1
fi

if $(echo $VERSION | grep -qo '\-[a-z]\+[0-9]*$'); then
    # Release candidate release
    CHANNEL="preview"
else
    # Stable releases should not have a suffix
    CHANNEL="stable"
fi

local_test_release="${TEST_RELEASE:-}"

# YourBase S3 artifacts Release
# TODO(ch1940): migrate to CATS
AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-}"
AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-}"
aws_disabled=true
[ -n "${AWS_ACCESS_KEY_ID}" -a -n "${AWS_SECRET_ACCESS_KEY}" ] && aws_disabled=false
if $aws_disabled; then
    echo "No AWS credentials, probably in Staging/Preview, giving up"
    # Non fatal, we won't get GH status errors when trying to release on Staging/Preview
    exit 0
fi

# Commit info
COMMIT="${YB_GIT_COMMIT:-}"
if [ -z "${COMMIT}" ]; then
    # If git is installed
    if hash git; then
        COMMIT="$(git rev-parse HEAD)"
    fi
fi

BUILD_COMMIT_INFO=""
if [ -n "${COMMIT}" ]; then
    BUILD_COMMIT_INFO=" -X 'main.commitSHA=$COMMIT'"
fi

umask 077

echo "Releasing ${CHANNEL} yb version ${VERSION} [${COMMIT}]..."

set -x

rm -rf release
mkdir -p release

OSLIST=( linux darwin )
ARCHLIST=( amd64 )

for os in "${OSLIST[@]}"
do
  for arch in "${ARCHLIST[@]}"
  do
    GOOS=${os} GOARCH=${arch} go build -ldflags "-X 'main.version=$VERSION' -X 'main.date=$(date -u '+%F-%T')' -X 'main.channel=$CHANNEL'${BUILD_COMMIT_INFO} -s -w" -o release/yb-${os}-${arch}
  done
done

(
  cd release

  for i in *
  do
    tar zcvf $i-${VERSION}.tgz $i
    rm $i
  done

    for i in *.tgz
    do
      if [ -z "${local_test_release}" ]; then
        if aws s3 ls s3://yourbase-artifacts/yb/${VERSION}/$i; then
            echo "A version for ${VERSION} already exists! Not releasing this version."
            exit 1
        fi

        aws s3 cp $i s3://yourbase-artifacts/yb/${VERSION}/
      else
        echo "Local test, would run:"
        echo aws s3 ls s3://yourbase-artifacts/yb/${VERSION}/$i
        echo aws s3 cp $i s3://yourbase-artifacts/yb/${VERSION}/
      fi
    done
)

# Equinox install procedure
# TODO Migrate to GoRelease (for GH Releases and Homebrew tap)

APP="app_gtQEt1zkGMj"
PROJECT="yb"
TOKEN="${RELEASE_TOKEN:-}"
RELEASE_KEY="${RELEASE_KEY:-}"

cleanup() {
    set -x
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
    set +x
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
