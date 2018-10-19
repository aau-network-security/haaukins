#!/usr/bin/env bash
f=./build/ntpd-linux-amd64
user=ntpd
hostname=sec02.lab.es.aau.dk
key=./travis_deploy_key

if [ -f $f ]; then
    chmod 600 $key
    echo -e "Host sec02\n\tHostname $hostname\n\tStrictHostKeyChecking no\n\tUser $user\n\tIdentityFile $key\n" >> ~/.ssh.config
    ssh sec02 pwd
else
    echo "Error: $f does not exist"
    exit 1
fi