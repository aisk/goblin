# Installation

Goblin is installed through the Go toolchain. Install [Go](https://go.dev/dl/)
1.20 or later, then verify that it is available:

```sh
$ go version
```

## Install with `go install`

```sh
$ go install github.com/aisk/goblin@latest
```

This downloads, builds, and installs `goblin` into `$GOBIN`, normally
`$HOME/go/bin` when `GOBIN` is unset. Ensure that directory is on your `PATH`.

## Verify the installation

```sh
$ goblin --help
```

If help text is printed, the installation is ready to use.

## Build from source

Alternatively, clone the repository and build it yourself:

```sh
$ git clone https://github.com/aisk/goblin.git
$ cd goblin
$ go build .
```

This produces a `goblin` executable in the current directory. While working
from a clone of the repository, you can also run it directly:

```sh
$ go run . run hello.goblin
```
