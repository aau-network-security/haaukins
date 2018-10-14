#!/bin/bash

GOARCH=( 386 amd64 )

GOOS=( linux darwin windows )
EXTENSION=( "" "" ".exe" )

APPDIR=( daemon client )
APPNAME=( ntpd ntp)

for goarch in "${GOARCH[@]}"
do
    for j in "${!GOOS[@]}"
    do
        goos="${GOOS[$j]}"
        ext="${EXTENSION[$j]}"
        for i in "${!APPDIR[@]}"
        do
            appdir="${APPDIR[$i]}"
            appname="${APPNAME[$i]}"
            env GOOS=$goos GOARCH=$goarch go build -o ./build/$appname-$goos-$goarch$ext github.com/aau-network-security/go-ntp/app/$appdir
        done
    done
done