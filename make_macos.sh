#!/bin/sh
rm -f one-api

CGO_ENABLED=1 GOARCH=amd64 GOOS=darwin go build -ldflags "-s -w" -o one-api

chmod u+x one-api
