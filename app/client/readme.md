# Client 
The command line interface for interacting with the daemon. 
Run `hkn -h` (using the binary) or `go run app/client/main.go -h` to print the possible commands.
The specification of the design of commands in the client can be found in [the wiki](https://github.com/aau-network-security/haaukins/wiki/CLI-specification).

## Optional Parameters
Optional parameters to the client is specified using environment variables.
- `HKN_HOST` overwrites the default host (default: `cli.sec-aau.dk`).
- `HKN_PORT` overwrites the default port (default: `5454`).
- `HKN_SSL_OFF` overwrites the default ssl options (default: `false`).
