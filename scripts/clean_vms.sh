#!/bin/bash

VMS=$(vboxmanage list vms | awk '{ print $1 }' | sed -n -e 's/"\([a-z0-9]\{32\}\)"/\1/p')

while read -r line; do
    vm=$(echo $line | cut -d ' ' -f 2)
    echo $vm
    vboxmanage controlvm $vm poweroff
    vboxmanage unregistervm $vm --delete
done <<< "$VMS"
