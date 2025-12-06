# goblin

![logo](https://repository-images.githubusercontent.com/650676396/21de79b8-27e8-4269-a4ad-d4c3b3b743f7)

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
