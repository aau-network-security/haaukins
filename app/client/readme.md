# Client 
The command line interface for interacting with the daemon. 
Run `ntp -h` (using the binary) or `go run app/client/main.go -h` to print the possible commands.
The specification of the design of commands in the client can be found in [the wiki](https://github.com/aau-network-security/go-ntp/wiki/CLI-specification).

## Optional Parameters
Optional parameters to the client is specified using environment variables.
- `NTP_HOST` overwrites the default host (default: `cli.sec-aau.dk`).
- `NTP_PORT` overwrites the default port (default: `5454`).
- `NTP_SSL_OFF` overwrites the default ssl options (default: `false`).
