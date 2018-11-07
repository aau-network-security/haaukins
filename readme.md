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

## Version release
In order to release a new version, run the `script/release/release.go` script as follows (choose depending on type of release):
```bash
$ go run script/release/release.go major 
$ go run script/release/release.go minor 
$ go run script/release/release.go patch 
```
The script will do the following:

- Bump the version in `VERSION` and commit to git
- Tag the current `HEAD` with the new version
- Create new branch(es), which depends on the type of release.
- Push to git

Travis automatically creates a release on GitHub and deploys on `sec02`.

Note: by default the script uses the `~/.ssh/id_rsa` key to push to GitHub.
You can override this settings by the `NTP_RELEASE_PEMFILE` env var.