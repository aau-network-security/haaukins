#!/bin/bash

VMS=$(vboxmanage list vms)

while read -r line; do
    line=$(echo $line | cut -d ' ' -f 2)
    vms_id=$(echo $line | awk -F[{}] '{print $2}')
    echo $vms_id
    vboxmanage controlvm $vms_id poweroff
    vboxmanage unregistervm $vms_id --delete
done <<< "$VMS"
