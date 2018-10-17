# Daemon
This is the core service of the NTP, and handles multi-hosting and tearing down of events.
Currently only one client exists for interacting with the daemon, which can be found in [client directory](../client).

## Configuration
By default the daemon will look for a `config.yml`-file in the directory it resides, alternatively you can make it look elsewhere by specifying the `-config=<path/to/config.yml>` flag.

The configuration format uses [YAML](https://learnxinyminutes.com/docs/yaml/), and an example of a configuration can be seen below.
``` yaml
host: event.ntp-aau.dk               # optional, defaults to 'localhost'
ova-directory: "/events"             # optional, defaults to './events'
events-directory: "/vms"             # optional, defaults to './vbox'
users-file: "/users.yml"             # optional, defaults to './users.yml'
exercises-file: "/exercises.yml"     # optional, defaults to './exercises.yml'
management:
  sign-key: "some_secret"          # required,  used for signing JSON Web Token (make it long and random)
  tls:
    cert-file: "cert.crt"        # optional, cert file for enabling TLS on management interface
    key-file: "key.key"          # optional, cert file for enabling TLS on management interface
docker-repositories:                 # optional, credentials for docker repositories
    - username: SomeUser
      password: SomePass
      serveraddress: some-registry.organization.com
```

