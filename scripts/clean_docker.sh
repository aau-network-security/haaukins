#!/usr/bin/env bash
# Remove all docker containers that have a UUID as name
docker ps -a --format '{{.Names}}' | grep -E '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' | xargs docker rm -f
# Remove all macvlan networks
docker network rm $(docker network ls -q -f "label=hkn")

# Prune entire docker
docker system prune -f

# Prune volumes
docker volume prune -f