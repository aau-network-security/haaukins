#!/usr/bin/env bash
f=./build/ntpd-linux-amd64
user=ntpd
hostname=sec02.lab.es.aau.dk
keyfile=./travis_deploy_key
deploy_path=/home/ntpd/daemon/ntpd

if [ -f $f ]; then
    echo "Deploying '$f' to '$hostname'"
    chmod 600 $keyfile
    ssh -i $keyfile -o StrictHostKeyChecking=no $user@$hostname sudo /bin/systemctl stop ntpd.service
    scp -i $keyfile -o StrictHostKeyChecking=no $f $user@$hostname:$deploy_path
    ssh -i $keyfile -o StrictHostKeyChecking=no $user@$hostname sudo /bin/systemctl start ntpd.service
else
    echo "Error: $f does not exist"
    exit 1
fi