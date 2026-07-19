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

For a typical Unix shell, add the following line to your shell profile if
`goblin` is not found after installation:

```sh
export PATH="$PATH:$(go env GOPATH)/bin"
```

## Verify the installation

```sh
$ goblin --help
```

If help text is printed, the installation is ready to use.

The CLI provides three subcommands:

| Command | Purpose |
| --- | --- |
| `goblin run file.goblin` | Interpret a source file |
| `goblin build-exe file.goblin` | Build a native executable |
| `goblin repl` | Start an interactive session |

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

Run the project's checks before contributing a change:

```sh
$ go test ./...
```

To verify the complete programs mirrored in the Book, run:

```sh
$ bash docs/check-examples.sh
```

## Updating

To update an installation made with `go install`, run the same command again:

```sh
$ go install github.com/aisk/goblin@latest
```
