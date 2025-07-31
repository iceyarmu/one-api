#!/bin/sh
currentShellPath=$(cd "$(dirname "$0")"; pwd)
cd "$currentShellPath"
if ! [ -f one-api ]; then
    echo "one-api not found, please run make_linux.command first"
    exit 1
fi
ssh vpn "sudo systemctl stop one-api; cp -f /home/ec2-user/one-api/one-api /home/ec2-user/one-api/one-api.bak; cp -f /home/ec2-user/one-api/one-api.db /home/ec2-user/one-api/one-api.db.bak"
rsync -av one-api vpn:/home/ec2-user/one-api/one-api
rsync -av web/dist/ vpn:/home/ec2-user/one-api/web/dist/
ssh vpn "sudo systemctl start one-api"