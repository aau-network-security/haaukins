# Daemon
This is the core service of Haaukins, and handles multi-hosting and tearing down of events.
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

### Exercise configuration
The `exercise.yml` contains the definition of the exercise library (view structure in [exercise.go](https://github.com/aau-network-security/go-ntp/blob/master/store/exercise.go#L36)). 
An example of an exercise definition:
```yaml
exercises:
  - name: SQL Injection
    tags:
    - sql
    docker:
    - image: registry.sec-aau.dk/aau/sql-client
      memoryMB: 50
      flag:
      - tag: sql-1
        name: Web server login
        env: INTERNAL_FLAG
        points: 5
      - tag: sql-2
        name: SQL injection
        env: DB_FLAG
        static: 334637ad-2c51-45dd-89d9-9b7c7155b366
        points: 15
    - image: registry.sec-aau.dk/aau/sql-server
      memoryMB: 85
      dns:
      - name: netsec-forum.dk
        type: A
      flag:
      - tag: sql-3
        name: Network sniffing
        env: FLAG
        default: 613ee1e1-ab15-4c0e-b324-3f2a6eee25f2
        points: 5
```

