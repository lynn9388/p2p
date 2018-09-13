# P2P

[![GoDoc](https://godoc.org/github.com/lynn9388/p2p?status.svg)](https://godoc.org/github.com/lynn9388/p2p)
[![Build Status](https://travis-ci.com/lynn9388/p2p.svg?branch=master)](https://travis-ci.com/lynn9388/p2p)

A simple P2P (peer-to-peer) network implementation.

## Install

Fist, use `go get` to install the latest version of the library:

```sh
go get -u github.com/lynn9388/p2p
```

Next, include this package in your application:

```go
import "github.com/lynn9388/p2p"
```

## Example

The code below shows how to create a new node.

```go
func main() {
	port := flag.Int("port", 9388, "port for server")
	flag.Parse()

	node := p2p.NewNode("localhost:" + strconv.Itoa(*port))
	node.JoinNetwork("localhost:9388")
	node.StartServer()
	node.Wait()
}
```

You can try it with `go run main.go -port PORT` in **example** directory. (Try to run several examples with different port.)

For more information you can check the [GoDoc](https://godoc.org/github.com/lynn9388/p2p)
