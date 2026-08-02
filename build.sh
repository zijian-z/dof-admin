#!/usr/bin/env bash
set -e

cd "$(dirname "$0")"

VERSION=$(git describe --tags --always 2>/dev/null || echo dev)
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS="-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.buildTime=$BUILD_TIME"

mkdir -p dist

echo "==> windows/amd64"
GOOS=windows GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/dof-admin-windows-amd64.exe ./cmd/admin

echo "==> linux/amd64"
GOOS=linux GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/dof-admin-linux-amd64 ./cmd/admin

echo "==> linux/arm64"
GOOS=linux GOARCH=arm64 go build -ldflags "$LDFLAGS" -o dist/dof-admin-linux-arm64 ./cmd/admin

ls -lh dist/
