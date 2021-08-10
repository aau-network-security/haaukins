#!/usr/bin/env bash
cd daemon/proto && protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative  daemon.proto

# add this value to proto file
# option go_package = "github.com/aau-network-security/haaukins/daemon/proto";

# install
# go get -u github.com/golang/protobuf/{proto,protoc-gen-go}
# go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
# go get -u google.golang.org/grpc