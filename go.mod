module github.com/aau-network-security/haaukins

go 1.13

require (
	github.com/PuerkitoBio/goquery v1.5.0
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/coreos/go-semver v0.3.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/docker/docker v1.13.1
	github.com/fsouza/go-dockerclient v1.4.4
	github.com/giantswarm/semver-bump v0.0.0-20181008095244-e8413386a9b8
	github.com/go-acme/lego v2.7.2+incompatible // indirect
	github.com/golang/protobuf v1.3.2
	github.com/google/uuid v1.1.1
	github.com/gorilla/websocket v1.4.1
	github.com/juju/errgo v0.0.0-20140925100237-08cceb5d0b53 // indirect
	github.com/logrusorgru/aurora v0.0.0-20190803045625-94edacc10f9b
	github.com/mholt/certmagic v0.7.3
	github.com/pkg/errors v0.8.1
	github.com/rs/zerolog v1.15.0
	github.com/schollz/progressbar v1.0.0
	github.com/shirou/gopsutil v2.18.12+incompatible
	github.com/spf13/cobra v0.0.5
	github.com/xenolf/lego v2.7.2+incompatible
	golang.org/x/crypto v0.0.0-20190927123631-a832865fa7ad
	google.golang.org/grpc v1.24.0
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/docker/docker => github.com/docker/engine v1.4.2-0.20190717161051-705d9623b7c1

replace github.com/schollz/progressbar v1.0.0 => github.com/schollz/progressbar/v2 v2.14.0
