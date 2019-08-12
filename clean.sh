#!

docker kill $(docker ps -q)
docker rm $(docker ps -q -a)
VBoxManage list runningvms | awk '{print $2;}' | xargs -I vmid VBoxManage controlvm vmid poweroff

rm -rf events/
