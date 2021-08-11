module github.com/aau-network-security/haaukins

go 1.15

replace github.com/docker/docker => github.com/docker/engine v1.4.2-0.20190717161051-705d9623b7c1

require (
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/coreos/go-semver v0.3.0
	github.com/docker/docker v1.4.2-0.20190927142053-ada3c14355ce
	github.com/fsouza/go-dockerclient v1.5.0
	github.com/giantswarm/semver-bump v0.0.0-20181008095244-e8413386a9b8
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/golang-jwt/jwt/v4 v4.0.0
	github.com/golang/protobuf v1.5.2
	github.com/gomarkdown/markdown v0.0.0-20210514010506-3b9f47219fe7
	github.com/google/uuid v1.1.2
	github.com/gorilla/websocket v1.4.1
	github.com/juju/errgo v0.0.0-20140925100237-08cceb5d0b53 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/logrusorgru/aurora v0.0.0-20191017060258-dc85c304c434
	github.com/microcosm-cc/bluemonday v1.0.15
	github.com/pkg/errors v0.8.1
	github.com/rs/zerolog v1.16.0
	github.com/shirou/gopsutil v2.19.9+incompatible
	github.com/sirupsen/logrus v1.4.2 // indirect
	github.com/spf13/cobra v0.0.5
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550
	golang.org/x/sys v0.0.0-20210616094352-59db8d763f22 // indirect
	google.golang.org/genproto v0.0.0-20210624195500-8bfb893ecb84 // indirect
	google.golang.org/grpc v1.38.0
	google.golang.org/protobuf v1.27.0
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v2 v2.2.4
)
