clean_proto:
	rm -rf daemon/proto/*.go
	rm -rf daemon/proto/*.json

clean_resources:
	bash scripts/clean_docker.sh
	bash scripts/clean_vms.sh


generate:
	buf mod update
	buf generate 

install:
	go get \
		github.com/bufbuild/buf/cmd/buf \
		github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
		github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
		google.golang.org/grpc/cmd/protoc-gen-go-grpc \
		google.golang.org/protobuf/cmd/protoc-gen-go

tidy:
	go mod tidy

run: 
	go run main.go --config=$(config)


test:
	go test -race ./...

