
<p align="center"> 
<img src=".github/logo/blue240px.png"  />
<div align="center">
<a href="https://beta.ntp-event.dk">
  <img src=https://img.shields.io/badge/platform-try%20haaukins-brightgreen>
  </a>
  <a href="https://travis-ci.com/aau-network-security/haaukins">
    <img src="https://travis-ci.com/aau-network-security/haaukins.svg?branch=master" alt="Build Status">
  </a>
  <a href="https://goreportcard.com/badge/github.com/aau-network-security/haaukins">
    <img src="https://goreportcard.com/badge/github.com/aau-network-security/haaukins?style=flat-square" alt="Go Report Card">
  </a>
  <a href="https://github.com/aau-network-security/haaukins/releases">
    <img src="https://godoc.org/github.com/aau-network-security/haaukins?status.svg" alt="GitHub release">
  </a>
   <a href="https://www.gnu.org/licenses/gpl-3.0Â">
    <img src="https://img.shields.io/badge/License-GPLv3-blue.svg?longCache=true&style=flat-square" alt="licence">
  </a>
  <div align ="center">
  <a href="https://github.com/aau-network-security/haaukins/issues">
  <img src=https://img.shields.io/github/issues/aau-network-security/haaukins?style=flat-square alt="issues">
  
  </a>
  <a href="https://github.com/aau-network-security/haaukins/network/members">
  <img src=https://img.shields.io/github/forks/aau-network-security/haaukins >
  </a>
  <a href="https://github.com/aau-network-security/haaukins/stargazers">
  <img src=https://img.shields.io/github/stars/aau-network-security/haaukins></a>
  </div>
  </div>
&nbsp;
<div align="center">
<h1>Haaukins </h1>
</div>

Haaukins is a highly accessible and automated virtualization platform for security education, it has three main components (Docker, Virtualbox and Golang), the communication and orchestration between the components managed using Go programming language. The main reason of having Go environment to manage and deploy something on Haaukins platform is that Go’s easy concurrency and parallelism mechanism. Want to get more insight about architecture of Haaukins visit [architecture page](https://github.com/aau-network-security/haaukins/wiki/Architecture-of-Haaukins)

Our primary aim to involve anyone who desire to learn capturing the flag concept in cyber security which is widely accepted approach to learn how to find vulnerability on a system. Despite of all existing platform, Haaukins provides its own virtualized environment to you with operating system which designed to find vulnerabilities

- [__Prerequisites__](#prerequisites)
- [__Installation__](#installation)
- [__Getting Dependencies__](#getting-dependencies)
- [__Testing__](#testing)
- [__Re-compile proto__](#re-compile-proto)
- [__Version release__](#version-release)
- [__Known issues__](#known-issues)
- [__Contributing__](#contributing)
- [__Event requests__](#event-requests)
  - [Event Requests Form](#event-requests-form)
- [__License__](#license)


## __Prerequisites__

The following dependencies are required and must be installed separately in order to run daemon in your local environment.

* Linux 
* Docker
* Go 1.13+

There is no prerequisites for installing client to your environment. 

> **Note**: Linux can be used in virtualized environment as well.

## __Installation__

To install daemon or client of Haaukins,  there are some options, via binary files, which are ready to use, visit [releases](https://github.com/aau-network-security/haaukins/releases) page.

More information about installation process, checkout following pages ; 

* [Installation for client](https://github.com/aau-network-security/haaukins/wiki/Installation) 


* [Configuration for daemon](https://github.com/aau-network-security/haaukins/wiki/Configuring-the-daemon)
  - There are some configuration files to configure daemon, those configuration files should be in same directory with the binary file that you have just downloaded from [releases](https://github.com/aau-network-security/haaukins/releases) page. 
  
  - Want to try daemon on you local computer with pre-configured vagrant file check out this [repo](https://github.com/aau-network-security/sec0x) for more information. 


## __Getting Dependencies__

Haaukins platform uses `go modules` since version [1.6.4](https://github.com/aau-network-security/haaukins/releases/tag/1.6.4), hence it is quite easy to manage dependencies, you just need to run `go mod download` 

## __Testing__

Make sure that you are in `$GOPATH/src/github.com/aau-network-security/haaukins/` directory, to run all test files, following command can be used

```bash
go test -v -short ./...
```

## __Re-compile proto__

Haaukins platform uses gRPC on communication of client and daemon, so after updating the protocol buffer specification (i.e. daemon/proto/daemon.proto), corresponding golang code generation is done by doing the following:
```bash
cd $GOPATH/src/github.com/aau-network-security/haaukins/daemon/
protoc -I proto/ proto/daemon.proto --go_out=plugins=grpc:proto
```

## __Version release__

In order to release a new version, run the `script/release/release.go` script as follows (choose depending on type of release):
```bash
$ go run $GOPATH/src/github.com/aau-network-security/haaukins/scripts/release/release.go major
$ go run $GOPATH/src/github.com/aau-network-security/haaukins/scripts/release/release.go minor
$ go run $GOPATH/src/github.com/aau-network-security/haaukins/scripts/release/release.go patch
```
The script will do the following:

- Bump the version in `VERSION` and commit to git
- Tag the current `HEAD` with the new version
- Create new branch(es), which depends on the type of release.
- Push to git

Travis automatically creates a release on GitHub and deploys on `server`.

Note: by default the script uses the `~/.ssh/id_rsa` key to push to GitHub.
You can override this settings by the `HKN_RELEASE_PEMFILE` env var.

## __Known issues__

Give a  moment and check known issues over [here](https://github.com/aau-network-security/haaukins/issues)

## __Contributing__

Haaukins is an open source project and built on the top of open-source projects. If you are interested, then you are welcome to contribute.

Check out the [Contributing Guide](.github/CONTRIBUTING.md) to get started.

## __Event requests__

As AAU, we believe in power of open source community and would like to offer test our platform for organizations and events , if you would like to get your own domain which will be assigned by us please fill following the form and contact us in advance.
After having your application, we will back to you as soon as possible 

### [Event Requests Form](https://docs.google.com/forms/d/e/1FAIpQLSeyFTle_29Afck00hSHPU5nWT7QMWYd42yB76ABIoIMCewdRg/viewform)


## __License__

[GNU](https://github.com/aau-network-security/haaukins/blob/master/LICENSE)

Copyright (c) 2019-present, Haaukins
