module github.com/aau-network-security/haaukins

go 1.13

replace github.com/docker/docker => github.com/docker/engine v1.4.2-0.20190717161051-705d9623b7c1

replace github.com/schollz/progressbar v1.0.0 => github.com/schollz/progressbar/v2 v2.14.0

require (
	github.com/PuerkitoBio/goquery v1.5.0
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/coreos/go-semver v0.3.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/docker/docker v1.4.2-0.20190927142053-ada3c14355ce
	github.com/fsouza/go-dockerclient v1.5.0
	github.com/giantswarm/semver-bump v0.0.0-20181008095244-e8413386a9b8
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/golang/protobuf v1.3.2
	github.com/google/uuid v1.1.1
	github.com/gorilla/websocket v1.4.1
	github.com/juju/errgo v0.0.0-20140925100237-08cceb5d0b53 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/logrusorgru/aurora v0.0.0-20191017060258-dc85c304c434
	github.com/pkg/errors v0.8.1
	github.com/rs/zerolog v1.16.0
	github.com/schollz/progressbar v1.0.0
	github.com/shirou/gopsutil v2.19.9+incompatible
	github.com/sirupsen/logrus v1.4.2 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/stretchr/testify v1.4.0 // indirect
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550
	golang.org/x/net v0.0.0-20191014212845-da9a3fd4c582 // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e // indirect
	golang.org/x/sys v0.0.0-20191018095205-727590c5006e // indirect
	google.golang.org/genproto v0.0.0-20191009194640-548a555dbc03 // indirect
	google.golang.org/grpc v1.24.0
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v2 v2.2.4
)
