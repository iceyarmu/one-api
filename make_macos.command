#!/bin/sh
currentShellPath=$(cd "$(dirname "$0")"; pwd)
cd "$currentShellPath"
rm -f one-api

cd web
bun install
bun run build
cd ..

CGO_ENABLED=1 GOARCH=amd64 GOOS=darwin go build -ldflags "-s -w" -o one-api

chmod u+x one-api
