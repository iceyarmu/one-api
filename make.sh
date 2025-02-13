#!/bin/sh

# mac amd64
CGO_ENABLED=1 GOARCH=amd64 GOOS=darwin go build -ldflags "-s -w" -o one-api

# linux amd64
# CGO_ENABLED=1 GOARCH=amd64 GOOS=linux go build -ldflags "-s -w" -o one-api
