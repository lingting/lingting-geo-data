#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
os=$(go env GOOS)
arch=$(go env GOARCH)

mkdir -p "$root/bin"
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$root/bin/sync-$os-$arch" "$root/cmd/sync"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "$root/bin/sync-linux-amd64" "$root/cmd/sync"
