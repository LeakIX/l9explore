# l9explore

[![GitHub Release](https://img.shields.io/github/v/release/LeakIX/l9explore)](https://github.com/LeakIX/l9explore/releases)
[![Follow on Twitter](https://img.shields.io/twitter/follow/leak_ix.svg?logo=twitter)](https://twitter.com/leak_ix)

l9explore is a plugin based tool doing deep exploration on a wide range of protocols.
It can be used to expose leaks, misconfigurations and vulnerabilities on any IP network.

It is the last layer in the l9 tool suite.

## Features

- Deep protocol exploration 
- Plugin based system
- Low memory/CPU footprint
- Multistage (WIP)

## Usage

### Explore services

```
l9explore service -h
```

Displays help for the list command.

|Flag           |Description  |
|-----------------------|-------------------------------------------------------|
|--max-threads    | Maximum number of threads |
|--only-leak      | Only display leaks and discard service events |
|--explore-timeout | Timeout for each plugin |
|--debug           | Displays developer information 
|--disable-explore-stage|Disable explore stage plugins ( schema or file list/content)|
|--exfiltrate-stage|Enable exfiltrate stage plugins ( dumps data to disk )|
|--option| Use `-o 'redis_password=test;...'` to pass options to plugins, check each plugin's documentation for details| 

## Installation Instructions

### From Binary

The installation is easy. You can download the pre-built binaries for your platform from the [Releases](https://github.com/LeakIX/l9explore/releases/) page.

This version has our [stock plugins](https://github.com/LeakIX/l9plugins) embedded.

```sh
▶ chmod +x l9explore-linux-64
▶ mv l9explore-linux-64 /usr/local/bin/l9explore
```

### From Source

```sh
▶ GO111MODULE=on go get -u -v github.com/LeakIX/l9explore/cmd/l9explore
▶ ${GOPATH}/bin/l9explore -h
```

## Running l9explore

l9explore speaks [l9format](https://github.com/LeakIX/l9format). It reads from stdin and outputs results on stdout.

An usual pipeline would be to use it with [l9tcpid](https://github.com/LeakIX/l9tcpid) to identify the protocols to explore. 

```sh
$ ulimit -n 4096 
$ sudo ip4scout random -r 25000 -p 27017,9200|l9tcpid service --deep-http --max-threads=2048|tee services.json|l9explore service --explore-timeout 5s -t 2048 -l|tee leaks.json|l9filter transform -i l9 -o human
2020/12/15 01:28:56 selected input : l9
2020/12/15 01:28:56 selected output :  human
2020/12/15 01:28:56 Recommended blacklist loaded
2020/12/15 01:28:56 30 networks in blacklist
2020/12/15 01:28:56 Loaded 2 ports to scan
2020/12/15 01:28:56 Using source port 7427
2020/12/15 01:28:56 Listening!
EVENT: leak IP: 200.104.19.66, PORT:9200, PROTO:elasticsearch, SSL:false
HTTP/1.1 200 OK
content-type: application/json; charset=UTF-8
content-length: 493

NoAuth
Cluster info:
...
EVENT: leak IP: 201.71.22.54, PORT:27017, PROTO:mongo, SSL:false
HTTP/1.0 200 OK
Connection: close
Content-Type: text/plain
Content-Length: 85
It looks like you are trying to access MongoDB over HTTP on the native driver port.
Found 1 collections:
Found collection "system.version"

EVENT: leak IP: 202.65.137.161, PORT:9200, PROTO:elasticsearch, SSL:false
HTTP/1.1 200 OK
content-type: application/json; charset=UTF-8
content-length: 493
NoAuth
Cluster info:
....
```

will :

- Run [ip4scout](https://github.com/LeakIX/ip4scout) to get a list of 9200,27017 open ports
- Run [l9tcpid](https://github.com/LeakIX/l9tcpid) to identify "real" elasticsearch and mongodb servers
  - And save that output to services.json
- Run l9explore and use each plugin against its protocol to output leak events.
  - And save that output to leaks.json
- Use [l9filter](https://github.com/LeakIX/l9filter) to translate l9format in a comprehensible output


## Creating plugins

Checkout the [l9plugin documentation](https://github.com/LeakIX/l9format/blob/master/l9plugin.md) on how to create your plugins.

