#!/bin/sh
currentShellPath=$(cd "$(dirname "$0")"; pwd)
cd "$currentShellPath"
rm -f one-api

cd web
bun install
bun run build
cd ..

docker run --rm --platform=linux/amd64 -v "$(pwd):/src" -w /src \
    -e HTTP_PROXY="http://host.docker.internal:1087" \
    -e HTTPS_PROXY="http://host.docker.internal:1087" \
    golang:latest bash -c \
    "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '-s -w' -o one-api"

chmod u+x one-api
