#!/bin/bash

set -x

APP="app_gtQEt1zkGMj"
PROJECT="yb"
TOKEN="${RELEASE_TOKEN}"
RELEASE_KEY="${RELEASE_KEY}"

if [ -z "${VERSION}" ]; then
  echo "Extracting version from tag ref ${YB_GIT_BRANCH}"
  VERSION="$(echo $YB_GIT_BRANCH | sed -e 's|refs/tags/||g')"
fi

if [ -z "${VERSION}" ]; then
  echo "No version provided, won't release"
  exit 1
fi

STABLE_TAGGED="$(echo $VERSION | grep -o '-stable')"

if [ -z "${CHANNEL}" ]; then
  echo "Channel not set, will release as unstable"
  CHANNEL="unstable"
fi

if [ "${STABLE_TAGGED}" == "-stable" ]; then
    CHANNEL="stable"
fi

umask 077

cleanup() {
    rv=$?
    rm -rf "$tmpkeyfile"
    exit $rv
}

tmpkeyfile="$(mktemp)"
trap "cleanup" INT TERM EXIT

KEY_FILE="${tmpkeyfile}"
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
	-ldflags "-X main.version=$VERSION -X 'main.date=$(date)' -X main.channel=$CHANNEL" \
	"github.com/yourbase/${PROJECT}"

# Now releasing to S3 and GH
echo "Releasing yb version ${VERSION}..."

rm -rf release
mkdir -p release

OSLIST=( linux darwin )
ARCHLIST=( amd64 )

for os in "${OSLIST[@]}"
do
  for arch in "${ARCHLIST[@]}"
  do
    GOOS=${os} GOARCH=${arch} go build -ldflags "-X 'main.version=$VERSION' -X 'main.buildDate=$(date)' -X 'main.channel=$CHANNEL'" -o release/yb-${os}-${arch}
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
    aws s3 ls s3://yourbase-artifacts/yb/${VERSION}/$i
    if [[ $? -eq 0 ]]; then
        echo "A version for ${VERSION} already exists! Not releasing this version."
        exit 1
    fi

    aws s3 cp $i s3://yourbase-artifacts/yb/${VERSION}/
  done
)

