# Installation

Goblin is written in Go, so the easiest way to install it is with the Go
toolchain.

## Prerequisites

You need [Go](https://go.dev/dl/) 1.20 or later. Verify your installation
with:

```sh
$ go version
```

## Install with `go install`

```sh
$ go install github.com/aisk/goblin@latest
```

This downloads, builds, and installs the `goblin` binary into
`$GOBIN` (which defaults to `$HOME/go/bin`). Make sure that directory is on
your `PATH`.

## Verify the installation

```sh
$ goblin --help
```

If you see the help output, you are ready to go.

## Build from source

Alternatively, clone the repository and build it yourself:

```sh
$ git clone https://github.com/aisk/goblin.git
$ cd goblin
$ go build .
```

This produces a `goblin` executable in the current directory.
