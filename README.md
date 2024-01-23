# goblin

Toy language built for fun.

## Installation

```sh
$ go install github.com/aisk/goblin/goblin@latest
```

## Hello world

```sh
$ cat hello.goblin
print("Hello, world!")

$ goblin hello.goblin > hello.go

$ go build hello.go

$ ./hello
"Hello, world!"
```

## Grammer

Take a look at `goblin.bnf`.

## About the Project

Goblin is &copy; 2023-2024 by [aisk](https://github.com/aisk).

### License

Vox is distributed by a [MIT license](https://github.com/aisk/goblib/tree/master/LICENSE).
