# go-ntp
The virtualisation platform for training of digital security.
`go-ntp` consists of two major components:
- [client](app/client/readme.md)
- [daemon](app/daemon/readme.md)

Not here for development, but just for controlling the platform? Visit the [Wiki](https://github.com/aau-network-security/go-ntp/wiki/Getting-started). 

## Running
First, install dependencies (requiring `go 1.7` or higher):
```bash
go get ./...
```

To run the client (visit the [readme](app/client/readme.md) for a description of the configuration):
```bash
go run app/client/main.go
```

To run the daemon (visit the [readme](app/daemon/readme.md) for a description of the configuration):
```bash
go run app/daemon/main.go
```

## Testing 
```bash
go test -v -short ./... 
```

## Compiling proto
After updating the protocol buffer specification (i.e. `daemon/proto/daemon.proto`), corresponding golang code generation is done by doing the following:
```bash
cd daemon
protoc -I proto/ proto/daemon.proto --go_out=plugins=grpc:proto
```
