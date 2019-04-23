#!/bin/bash

set -eu

APP="app_gtQEt1zkGMj"
PROJECT="artificer"
VERSION=$1

equinox release \
        --version=$VERSION \
        --platforms="darwin_amd64 linux_amd64" \
        --signing-key=${HOME}/equinox.key  \
        --app="$APP" \
        --token="$(cat ${HOME}/equinox.token)" \
	"github.com/microclusters/${PROJECT}" \
	-ldflags "-X main.version=$VERSION -X 'main.date=$(date)'"
