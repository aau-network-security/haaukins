#!/bin/bash

GOARCH=( 386 amd64 )
GOOS=( linux darwin windows )

APPDIR=( daemon client )
APPNAME=( ntpd ntp)

for goarch in "${GOARCH[@]}"
do
    for goos in "${GOOS[@]}"
    do
        for i in "${!APPDIR[@]}"
        do
            appdir="${APPDIR[$i]}"
            appname="${APPNAME[$i]}"
            env GOOS=$goos GOARCH=$goarch go build -o ./build/$appname-$goos-$goarch github.com/aau-network-security/go-ntp/app/$appdir
        done
    done
done