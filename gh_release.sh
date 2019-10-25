#!/bin/bash

# After finishing https://github.com/yourbase/buildagent/tree/metadata, we'll have enought env variables to fill this and use this function, but inside `./release.sh`

function gh_upload_release_asset(repo, owner, tag, github_api_token, filename) {
    # Got from https://gist.github.com/stefanbuck/ce788fee19ab6eb0b4447a85fc99f447
    # Define variables.
    GH_API="https://api.github.com"
    GH_REPO="$GH_API/repos/$owner/$repo"
    GH_TAGS="$GH_REPO/releases/tags/$tag"
    AUTH="Authorization: token $github_api_token"
    WGET_ARGS="--content-disposition --auth-no-challenge --no-cookie"
    CURL_ARGS="-LJO#"

    # if [[ "$tag" == 'LATEST' ]]; then
    #   GH_TAGS="$GH_REPO/releases/latest"
    # fi

    # Validate token.
    curl -o /dev/null -sH "$AUTH" $GH_REPO || { echo "Error: Invalid repo, token or network issue!";  exit 1; }

    # Read asset tags.
    response=$(curl -sH "$AUTH" $GH_TAGS)

    # Get ID of the release.
    eval $(echo "$response" | grep -m 1 "id.:" | grep -w id | tr : = | tr -cd '[[:alnum:]]=')
    [ "$id" ] || { echo "Error: Failed to get release id for tag: $tag"; echo "$response" | awk 'length($0)<100' >&2; exit 1; }
    release_id="$id"

    # Get ID of the asset based on given filename.
    id=""
    eval $(echo "$response" | grep -C1 "name.:.\+$filename" | grep -m 1 "id.:" | grep -w id | tr : = | tr -cd '[[:alnum:]]=')
    assert_id="$id"
    if [ "$assert_id" = "" ]; then
        echo "No need to overwrite asset"
    else
        echo "Deleting asset($assert_id)... "
        curl "$GITHUB_OAUTH_BASIC" -X "DELETE" -H "Authorization: token $github_api_token" "https://api.github.com/repos/$owner/$repo/releases/assets/$assert_id"
    fi

    # Upload asset
    echo "Uploading asset... "

    # Construct url
    GH_ASSET="https://uploads.github.com/repos/$owner/$repo/releases/$release_id/assets?name=$(basename $filename)"

    curl "$GITHUB_OAUTH_BASIC" --data-binary @"$filename" -H "Authorization: token $github_api_token" -H "Content-Type: application/octet-stream" $GH_ASSET
}


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
        chmod -x release/yb-${os}-${arch}-${CHANNEL}.xz
        mv release/yb-${os}-${arch}-${CHANNEL}.xz release/yb-${VERSION}-${os}-${arch}-${CHANNEL}.xz
        echo "Please upload release/yb-${VERSION}-${os}-${arch}-${CHANNEL}.zx to a GH release"
    else
        zip -v release/yb-${VERSION}-${os}-${arch}-${CHANNEL}.zip release/yb-${os}-${arch}-${CHANNEL}
        rm release/yb-${os}-${arch}-${CHANNEL} 
        echo "Please upload release/yb-${VERSION}-${os}-${arch}-${CHANNEL}.zip to a GH release"
    fi
  done
done
