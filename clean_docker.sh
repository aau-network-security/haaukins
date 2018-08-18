# Remove all docker containers
docker rm -f $(docker ps -a -q)
# Remove all macvlan networks
docker network rm $(docker network ls | grep macvlan | awk '{ print $1 }')
