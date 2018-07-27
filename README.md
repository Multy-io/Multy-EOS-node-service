# Multy EOS node service
EOS node gRPC API for [Multy](multy.io) backend

## Installation
First, clone the repo:
```bash
git clone https://github.com/Multy-io/Multy-EOS-node-service
cd Multy-EOS-node-service
```
Then sync the dependencies using [govendor](https://github.com/kardianos/govendor):
```bash
govendor sync
```
And build:
```bash
go build -o multy-steem
```

## Usage
Check out help (notice optional initialization using environment variables):
```bash
$ ./multy-eos -h
NAME:
   multy-eos - eos node gRPC API for Multy backend

USAGE:
   multy-eos [global options] command [command options] [arguments...]

VERSION:
   v0.2 (commit: , branch: HEAD, buildtime: 2018-07-10T12:24:40+0300)

AUTHOR:
   vovapi

COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --host value     hostname to bind to [$MULTY_EOS_HOST]
   --port value     port to bind to (default: "8080") [$MULTY_EOS_PORT]
   --node value     node api address [$MULTY_EOS_NODE]
   --account value  eos account for user registration [$MULTY_EOS_ACCOUNT]
   --key value      active key for specified user for user registration [$MULTY_EOS_KEY]
   --help, -h       show help
   --version, -v    print the version

```
## API
Checkout events in [proto/eos.proto](eos.proto):

## TODO
* Graceful shutdown
