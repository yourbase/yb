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

if [ -z "${CHANNEL}" ]; then
  echo "Channel not set, will release as unstable"
  CHANNEL="unstable"
fi

if [ "${CHANNEL}" == "preview" ]; then
  echo "Channel is preview, setting version to timestamp"
  VERSION="$(date +"%Y%m%d%H%M%S")"
elif [ "${CHANNEL}" == "stable" ]; then
  echo "To promote to stable, use the equinox release tool:"
  echo '$ equinox publish --token $TOKEN --app $APP  --channel stable --release $VERSION'
  exit 0
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
	-ldflags "-X main.version=$VERSION -X 'main.date=$(date)'" \
	"github.com/yourbase/${PROJECT}"

if [ "${CHANNEL}" == "preview" ]; then
    exit 0
fi

# Now releasing to S3
echo "Releasing yb version ${VERSION}..."

rm -rf release
mkdir -p release

OSLIST=( linux darwin )
ARCHLIST=( amd64 )

for os in "${OSLIST[@]}"
do
  for arch in "${ARCHLIST[@]}"
  do
    GOOS=${os} GOARCH=${arch} go build -ldflags "-X 'github.com/yourbase/yb/main.version=$VERSION' -X 'github.com/yourbase/yb/main.buildDate=$(date)'" -o release/yb-${os}-${arch}
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

