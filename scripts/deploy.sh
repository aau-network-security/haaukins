#!/usr/bin/env bash
f=./build/ntpd-linux-amd64
user=ntpd
hostname=sec02.lab.es.aau.dk
keyfile=./travis_deploy_key

if [ -f $f ]; then
    echo "Deploying '$f' to '$hostname'"
    chmod 600 $keyfile
    ssh -i $keyfile $user@$hostname -o StrictHostKeyChecking=no pwd
else
    echo "Error: $f does not exist"
    exit 1
fi