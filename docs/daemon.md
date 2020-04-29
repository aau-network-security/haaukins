# Daemon
This is the core service of Haaukins, and handles multi-hosting and tearing down of events.
Currently only one client exists for interacting with the daemon, which can be found in [client directory](../client).

## Configuration
By default the daemon will look for a `config.yml`-file in the directory it resides, alternatively you can make it look elsewhere by specifying the `-config=<path/to/config.yml>` flag.

The configuration format uses [YAML](https://learnxinyminutes.com/docs/yaml/), and an example of a configuration can be seen below.
``` yaml
host:
  http: ntp-event.dk
  grpc: cli.sec-aau.dk
port:
  insecure: 8080
  secure: 8081
ova-directory: "/scratch/ova"
sign-key: ...
tls:
  enabled: true
  acme:
    email: ...
    api-key: ...
    development: false
docker-repositories:
- username: ...
  password: ...
  serveraddress: <registry URL>
```

### Exercise configuration
The `exercise.yml` contains the definition of the exercise library (view structure in [exercise.go](https://github.com/aau-network-security/haaukins/blob/master/store/exercise.go#L36)). 
An example of an exercise definition:
```yaml
exercises:
  - name: Cross-site Request Forgery
    tags:
    - csrf
    docker:
    - image: <registry host>/aau/csrf
      dns:
      - name: formalbank.com
        type: A
      memoryMB: 80
      flag:
      - tag: csrf-1
        name: Cross-site Request Forgery
        env: APP_FLAG
        points: 12
```

