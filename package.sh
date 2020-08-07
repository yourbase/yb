#!/bin/bash

set -euo pipefail

source variables.sh

PKG_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "$PKG_DIR"
}
trap cleanup EXIT


OSLIST=( linux darwin )
ARCHLIST=( amd64 )

for os in "${OSLIST[@]}"
do
  for arch in "${ARCHLIST[@]}"
  do
    BUNDLE="yb_${VERSION}_${os}_${arch}"
    echo "Building yb package $VERSION in $PKG_DIR/$BUNDLE..."

    GOOS="$os" GOARCH="$arch" go build -ldflags \
    "-X 'main.version=$VERSION' -X 'main.date=$(date -u '+%F-%T')' -X 'main.channel=$CHANNEL'${BUILD_COMMIT_INFO} -s -w" \
    -o "$PKG_DIR/$BUNDLE/yb"

    # CATS
    out="$( pwd )"
    ( cd "$PKG_DIR" && zip -r "$out/$BUNDLE.zip" "$BUNDLE" )

    # TODO(ch1844): Remove after being sure no build server needs this anymore
    mv "$PKG_DIR/$BUNDLE/yb" "$PKG_DIR/$BUNDLE/yb-${os}-${arch}"
    tar -C "$PKG_DIR/$BUNDLE" -zcvf "$out/yb-${os}-${arch}-${VERSION}.tgz" "yb-${os}-${arch}"

  done
done
