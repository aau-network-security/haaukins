#!/usr/bin/env bash
f=dist/hknd_linux_amd64/hknd
user=ntpd
hostname=sec02.lab.es.aau.dk
keyfile=./travis_deploy_key
deploy_path=/home/ntpd/daemon/hknd

if [ -f $f ]; then
    echo "Deploying '$f' to '$hostname'"
    chmod 600 $keyfile
    ssh -i $keyfile -o StrictHostKeyChecking=no $user@$hostname sudo /bin/systemctl stop hknd.service
    scp -i $keyfile -o StrictHostKeyChecking=no $f $user@$hostname:$deploy_path
    ssh -i $keyfile -o StrictHostKeyChecking=no $user@$hostname sudo /bin/systemctl start hknd.service
else
    echo "Error: $f does not exist"
    exit 1
fi