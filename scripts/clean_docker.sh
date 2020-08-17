#!/usr/bin/env bash

docker kill $(docker ps -q --filter "label=api")
# Removing killed containers which have label of "hkn"
docker rm $(docker ps -q -a --filter "label=api" --filter status=exited)
# Remove all macvlan networks
docker network rm $(docker network ls -q -f "label=api")
