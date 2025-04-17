#!/bin/sh
currentShellPath=$(cd "$(dirname "$0")"; pwd)
cd "$currentShellPath"
if ! [ -f one-api ]; then
    echo "one-api not found, please run make_linux.command first"
    exit 1
fi
ssh vpn "sudo systemctl stop one-api"
rsync -av one-api vpn:/home/ec2-user/one-api/one-api
ssh vpn "sudo systemctl start one-api"