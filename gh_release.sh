#!/bin/bash

set -x

PROJECT="yb"

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
fi

umask 077

# Now releasing to GH (but need manual upload)
echo "Releasing local yb version ${VERSION}..."

rm -rf release
mkdir -p release

OSLIST=( linux darwin )
ARCHLIST=( amd64 )

for os in "${OSLIST[@]}"
do
  for arch in "${ARCHLIST[@]}"
  do
    GOOS=${os} GOARCH=${arch} go build -ldflags "-X 'main.version=$VERSION' -X 'main.buildDate=$(date)' -X 'main.channel=$CHANNEL'" -o release/yb-${os}-${arch}-${CHANNEL}
    if [ "$os" == "linux" ]; then
        xz -ve release/yb-${os}-${arch}-${CHANNEL}
        echo "Please upload release/yb-${os}-${arch}-${CHANNEL}.zx to a GH release"
    else
        zip -v release/yb-${os}-${arch}-${CHANNEL} release/yb-${os}-${arch}-${CHANNEL}
        rm release/yb-${os}-${arch}-${CHANNEL} 
        echo "Please upload release/yb-${os}-${arch}-${CHANNEL}.zip to a GH release"
    fi
  done
done
