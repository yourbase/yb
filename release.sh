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

local_test_release="${2:-}"

if [ -z "${local_test_release}" ]; then
    # YourBase S3 artifacts Release
    # TODO migrate to CATS
    aws_disabled=true
    [ -n "${AWS_ACCESS_KEY_ID}" -a -n "${AWS_SECRET_ACCESS_KEY}" ] && aws_disabled=false
    if $aws_disabled; then
        echo "No AWS credentials, probably in Staging/Preview, giving up"
        # Non fatal, we won't get GH status errors when trying to release on Staging/Preview
        exit 0
    fi
fi

# Commit info
COMMIT="-"
if [ -z "${local_test_release}" ]; then
    COMMIT="${YB_GIT_COMMIT}"
else
    # If git is installed
    if hash git; then
        COMMIT="$(git rev-list -n1 HEAD)"
    fi
fi

BUILD_COMMIT_INFO=""
if [ "${COMMIT}" != "-" ]; then
    BUILD_COMMIT_INFO=" -X 'main.commitSHA=$COMMIT'"
fi

umask 077

echo "Releasing ${CHANNEL} yb version ${VERSION}..."

set -x

rm -rf release
mkdir -p release

OSLIST=( linux darwin )
ARCHLIST=( amd64 )

for os in "${OSLIST[@]}"
do
  for arch in "${ARCHLIST[@]}"
  do
    GOOS=${os} GOARCH=${arch} go build -ldflags "-X 'main.version=$VERSION' -X 'main.date=$(date)' -X 'main.channel=$CHANNEL'${BUILD_COMMIT_INFO}" -o release/yb-${os}-${arch}
  done
done

(
  cd release

  for i in *
  do
    tar zcvf $i-${VERSION}.tgz $i
    rm $i
  done

  if [ -z "${local_test_release}" ]; then
    for i in *.tgz
    do
      aws s3 ls s3://yourbase-artifacts/yb/${VERSION}/$i
      if [[ $? -eq 0 ]]; then
          echo "A version for ${VERSION} already exists! Not releasing this version."
          exit 1
      fi

      aws s3 cp $i s3://yourbase-artifacts/yb/${VERSION}/
    done
  fi
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
    exit $rv
}

tmpkeyfile="$(mktemp)"
trap "cleanup" INT TERM EXIT

KEY_FILE="${tmpkeyfile}"

equinox_disabled=true

if [ -n "${local_test_release}" ]; then
    set +x
    echo -e "Would run:
./equinox release
        --version=$VERSION
        --platforms='darwin_amd64 linux_amd64'
        --signing-key='${KEY_FILE}'
        --app='$APP'
        --token='${TOKEN}'
        --channel='${CHANNEL}'
    --
    -ldflags '-X main.version=$VERSION -X 'main.date=$(date)' -X 'main.channel=$CHANNEL'${BUILD_COMMIT_INFO}'
    'github.com/yourbase/${PROJECT}'"

    exit 0
fi

[ -n "${RELEASE_KEY}" -a -n "${TOKEN}" ] && equinox_disabled=false
if $equinox_disabled; then
    echo "No Equinox credentials, probably in Staging/Preview, giving up"
    # Non fatal, we won't get GH status errors when trying to release on Staging/Preview
    exit 0
fi

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
    -ldflags "-X main.version=$VERSION -X 'main.date=$(date)' -X 'main.channel=$CHANNEL'${BUILD_COMMIT_INFO}" \
    "github.com/yourbase/${PROJECT}"

