#!/usr/bin/env bash
cd daemon && protoc -I proto/ proto/daemon.proto --go_out=plugins=grpc:proto