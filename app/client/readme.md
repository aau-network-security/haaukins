# Client 
The command line interface for interacting with the daemon. 
Run `hkn -h` (using the binary) or `go run app/client/main.go -h` to print the possible commands.
The specification of the design of commands in the client can be found in [the wiki](https://github.com/aau-network-security/haaukins/wiki/CLI-specification).

* [Getting Started](#getting-started)
  * [Flags](#flags)
  * [Invite user](#invite-user)
  * [Start an event](#start-an-event)
  * [List events](#list-events)
  * [List event teams](#list-event-teams)
  * [Stop an event]($stop-an-event)
  * [Restart team lab](#restart-team-lab)
* [Optional Parameters](#optional-parameters)

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
### __Flags__

```bash
hkn event create boot -n "Boot " -a 5 -c 10 -e xss,scan,hb,phish -f kali
```
**_Regarding to flags on command_**

```
(-n or --name): Title of the event
(-a or --available): Requested number of labs
(-c or --capacity): Capacity of requested event
(-e or --exercises): Set of exercise tags (which are defined under exercise.yml file)
(-f or --frontend) : Virtual machine to use
```
### __Invite user__

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


### __Start an event__

Starting an event is really easy when you have access to server [see more information how to gain access](#how-to-gain-grant),

```console
$ hkn event create example -n "Example Event " -a 10 -c 20 -e xss,scan,hb,phish -f kali

60% |████████████████████████                |  [32s:4s]


```
__Example run of starting an event command:__

<a href="https://asciinema.org/a/YdCOBU82yHrt5Y90JdFgGtNM0" target="_blank"><img src="https://asciinema.org/a/YdCOBU82yHrt5Y90JdFgGtNM0.svg" /></a>


### __List events__

```console
$ hkn event list

EVENT TAG     NAME                  # TEAM   # EXERCISES   CAPACITY   CREATION TIME
natctfevent   National CTF event    1         4             2          2019-05-08 10:40:26
boot          Boot                  4         4             10         2018-12-11 23:16:01
aauctf        CTF event             7         4             10         2019-07-09 08:38:17

```

### __List Event Teams__

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

### __Stop an event__

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

### __Restart Team Lab__

```console
$ hkn event restart -h

Restart lab for a team

Usage:
  hkn event restart [event tag] [team id] [flags]

Examples:
hkn event restart esboot d11eb89b

Flags:
  -h, --help   help for restart


```

## __Optional Parameters__
Optional parameters to the client is specified using environment variables.
- `HKN_HOST` overwrites the default host (default: `cli.sec-aau.dk`).
- `HKN_PORT` overwrites the default port (default: `5454`).
- `HKN_SSL_OFF` overwrites the default ssl options (default: `false`).
