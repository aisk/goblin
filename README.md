# goblin

![logo](https://i.redd.it/jq6wig9ybssb1.png)

Toy language built for fun.

## Installation

```sh
$ go install github.com/aisk/goblin/command/goblin@latest
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

Take a look at [goblin.bnf](https://github.com/aisk/goblin/tree/master/goblin.bnf).

## About the Project

Goblin is &copy; 2023-2024 by [aisk](https://github.com/aisk).

### License

Vox is distributed by a [MIT license](https://github.com/aisk/goblin/tree/master/LICENSE).
