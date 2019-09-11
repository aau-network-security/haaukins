
<p align="center"> <img src="https://htmmhw.am.files.1drv.com/y4myCMonnjXvUy8ekRggNEz7_4RosFe7MF6cG8BDb13RfIaOfEi8a7c6LU37vbLT8OFBHc_I0uEurjLLLSHjqHgSyySLW6f6TdbcNqSJN7j4TCM46UG_z5T15M6kjxlp8bi9fHrOLqKrBoKV6vTpSCL8TjmqIVYGwWxPFZo0VHuf12fSaTZG1prJPwo8bDJzEIl7WcTvlxmgCDwX4--M7IrVw?width=700&height=200&cropmode=none" width="700" height="200" />
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
  </a
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
  <img src=https://img.shields.io/github/stars/aau-network-security/haaukins>
  </div>
  </div>
&nbsp;

Haaukins is a highly accessible and automated virtualization platform for security education, it has three main components (Docker, Virtualbox and Golang), the communication and orchestration between the components managed using Go programming language. The main reason of having Go environment to manage and deploy something on Haaukins platform is that Go’s easy concurrency and parallelism mechanism.
&nbsp;

=======
# Haaukins

[![Build Status](https://travis-ci.com/aau-network-security/haaukins.svg?branch=master)](https://travis-ci.com/aau-network-security/haaukins)  [![Go Report Card](https://goreportcard.com/badge/github.com/aau-network-security/haaukins)](https://goreportcard.com/report/github.com/aau-network-security/haaukins) [![GoDoc](https://godoc.org/github.com/aau-network-security/haaukins?status.svg)](https://godoc.org/github.com/aau-network-security/haaukins) [![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

The virtualisation platform for training of digital security.

`Haaukins` consists of two major components:
- [client](app/client/readme.md)
- [daemon](app/daemon/readme.md)

Our primary aim to involve anyone who desire to learn capturing the flag concept in cyber security which is widely accepted approach to learn how to find vulnerability on a system. Despite of all existing platform, Haaukins provides its own virtualized environment to you with operating system which designed to find vulnerabilities

* [Installation](#installation)
* [Getting Dependencies](#getting-dependencies)
* [Getting Started](#getting-started)
  * [Invite user](#invite-user)
  * [Start an event](#start-an-event)
  * [List events](#list-events)
  * [List event teams](#list-event-teams)
  * [Stop an event]($stop-an-event)
  * [Restart team lab](#restart-team-lab)
* [Haaukins architecture](#haaukins-architecture)
* [Testing](#testing)
* [Re-compile proto](#re-compile-proto)
* [Known issues](#known-issues)
* [Contributing](#contributing)
* [Event requests](#event-requests)
* [License](#license)


## __Installation__

### Prerequisites

The following dependencies are required and must be installed separately in order to run daemon in your local environment.

* Linux
* Docker
* Go 1.11+

> **Note**: Linux can be used in virtualized environment as well.

### Install

To install daemon or client of Haaukins,  there are some options, via binary files, which are ready to use, visit [Releases](https://github.com/aau-network-security/haaukins/releases) page.

#### Client

Just download proper version of binary then locate it to somewhere that the path of binary defined on your bash profile.

Need more information on installing Haaukins client, have a look over [here](https://github.com/aau-network-security/haaukins/wiki/Installation)

#### Daemon

In order to run daemon on your local environment, there are couple of preliminary information that you should keep in mind.

First of all, all [prerequisites](#Prerequisites) should be installed and [GOPATH & GOROOT](https://golang.org/doc/install) set properly.

There are some configuration files to configure daemon, those configuration files should be in same directory with the binary file that you have just downloaded from [releases](https://github.com/aau-network-security/haaukins/releases) page. (More info can be found [here](https://github.com/aau-network-security/haaukins/wiki/Configuring-the-daemon))

If you would like to run Haaukins in development stage than you should specify some environment variables before running `app/daemon/main.go`.

## __Getting Dependencies__

Install dependencies (requiring `go 1.7` or higher):

```bash
go get ./...
```
#### Running for development purposes

Make sure that you have pre-configured box installed in your local computer by given instructions from the repo [sec0x](https://github.com/aau-network-security/sec0x)

Set some required environment variables into your bash profile before running the box under vagrant

```
export VMDKS=...
export VMDKS=...
export CONFIGS=...
export FRNTENDS=....
```
After having set of given variables in your bash profile, you are good to go !

```
HKN_HOST=localhost HKN_SSL_OFF=true go run app/client/main.go event create boot -n "Boot " -a 5 -c 10 -e xss,scan,hb,phish -f kali

```
**Regarding to flags on command *

```
(-n or --name): Title of the event
(-a or --available): Requested number of labs
(-c or --capacity): Capacity of requested event
(-e or --exercises): Set of exercise tags (which are defined under exercise.yml file)
(-f or --frontend) : Virtual machine to use
```


## __Getting Started__


```
Usage:

hkn [command] [...]

Usage:
  hkn [command]

Available Commands:
  event       Actions to perform on events
  events      List events
  exercise    Actions to perform on exercises
  exercises   List exercises
  frontend    Actions to perform on frontends
  frontends   List available frontends
  help        Help about any command
  host        Actions to perform on host
  team        Actions to perform on teams
  user        Actions to perform on users
  version     Print version

Flags:
  -h, --help   help for hkn

```

### Invite user

Users have right to create,update,stop and list events should be invited by us, the invitation process take place in following format

```console 
$ hkn user invite --help
Create key for inviting other users (superuser only)

Usage:
  hkn user invite [flags]

Examples:
hkn user invite --superuser

Flags:
  -h, --help         help for invite
  -s, --super-user   indicates if the signup key will create a super user

```
When we created an invitation, random string will be produced in our console, etc `073a5ba4-69d7-sn34-8a4c-24124145` then that key can be used by user as sign up key, to get help how to do it, try following command 

```console 
$ hkn user signup --help
Signup as user

Usage:
  hkn user signup [flags]

Examples:
hkn user signup

Flags:
  -h, --help   help for signup

```
When you type `hkn user signup`, Haaukins will ask following field to create a user on server who may have limited or full access depending on situation. 
```console 
$ hkn user signup 

  Signup key: <the-key-given-by-administrator>
  Username: 
  Password:
  Password (again):

```


### Start an event

Starting an event is really easy when you have access to server [see more information how to gain access](#how-to-gain-grant),

```console
$ hkn event create example -n "Example Event " -a 10 -c 20 -e xss,scan,hb,phish -f kali

60% |████████████████████████                |  [32s:4s]


```
__Example run of starting an event command:__

<a href="https://asciinema.org/a/YdCOBU82yHrt5Y90JdFgGtNM0" target="_blank"><img src="https://asciinema.org/a/YdCOBU82yHrt5Y90JdFgGtNM0.svg" /></a>


### List events

```console
$ hkn event list

EVENT TAG     NAME                  # TEAM   # EXERCISES   CAPACITY   CREATION TIME
natctfevent   National CTF event    1         4             2          2019-05-08 10:40:26
boot          Boot                  4         4             10         2018-12-11 23:16:01
aauctf        CTF event             7         4             10         2019-07-09 08:38:17

```

### List Event Teams

```console
$ hkn event teams --help

  Get teams for a event

  Usage:
   hkn event teams [event tag] [flags]

  Examples:
  hkn event teams esboot

  Flags:
   -h, --help   help for teams

```

### Stop an event

```console
$ hkn event stop --help

  Stop event

  Usage:
    hkn event stop [event tag] [flags]

  Examples:
  hkn event stop esboot

  Flags:
    -h, --help   help for stop

```

### Restart Team Lab

```console
$hkn event restart -h

Restart lab for a team

Usage:
  hkn event restart [event tag] [team id] [flags]

Examples:
hkn event restart esboot d11eb89b

Flags:
  -h, --help   help for restart


```

## __Haaukins Architecture__

Haaukins consists of three main components which are Docker, Virtualbox and Go Programming. The internal communication and design of Haaukins can be viewed over [architecture page](https://github.com/aau-network-security/haaukins/wiki/Architecture-of-Haaukins)

## __Testing__

Make sure that you are in `.../aau-network-security/haaukins/` directory, to run all test files, following command can be used

```bash
go test -v -short ./...
```

## __Re-compile proto__

Haaukins platform uses gRPC on communication of client and daemon, so after updating the protocol buffer specification (i.e. daemon/proto/daemon.proto), corresponding golang code generation is done by doing the following:
```bash
cd daemon
protoc -I proto/ proto/daemon.proto --go_out=plugins=grpc:proto
```

## __Version release__

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

Travis automatically creates a release on GitHub and deploys on `server`.

Note: by default the script uses the `~/.ssh/id_rsa` key to push to GitHub.
You can override this settings by the `HKN_RELEASE_PEMFILE` env var.

## __Known issues__

Give a  moment and check known issues over [here](https://github.com/aau-network-security/haaukins/issues)

## __Contributing__

Haaukins is an open source project and built on the top of open-source projects. If you are interested, then you are welcome to contribute.

Check out the [Contributing Guide](./github/CONTRIBUTING.md) to get started.

## __Event requests__

As AAU, we believe in power of open source community and would like to offer test our platform for organizations and events , if you would like to get your own domain which will be assigned by us please fill following the form and contact us in advance.
After having your application, we will back to you as soon as possible 

### [Event Requests Form](https://docs.google.com/forms/d/e/1FAIpQLSeyFTle_29Afck00hSHPU5nWT7QMWYd42yB76ABIoIMCewdRg/viewform)


## __License__

[GNU](https://github.com/aau-network-security/haaukins/blob/master/LICENSE)

Copyright (c) 2018-present, Haaukins
