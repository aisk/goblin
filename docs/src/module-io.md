# io

The io module exports the stream shapes as the traits `Reader` and `Writer`.
Every standard library API that consumes a stream, such as an HTTP request
body, `exec.Command(stdout=...)`, or the `dest=` keyword of `csv.write_all` and
the compression and archive modules, accepts a value that implements the trait
as well as one that simply defines the methods.

| Trait | Required | Optional | Contract |
| --- | --- | --- | --- |
| `Reader` | `read(self, size)` | `close(self)` | Return at most `size` bytes as Bytes or str. An empty chunk or `nil` ends the stream. |
| `Writer` | `write(self, data)` | `close(self)` | Receive a Bytes chunk and return the number of bytes written, or `nil` for all of them. |

`close` does nothing by default. Consumers that own a stream call it when they
finish; writer consumers never do, because the stream's owner decides when it
ends.

~~~goblin
import "io"
import "csv"

type Collector(chunks) {
    impl io.Writer {
        func write(self, data) {
            self.chunks.push(data.decode())
            return data.size
        }
    }
}

var out = Collector([])
csv.write_all([["name", "score"], ["Ada", "10"]], dest=out)
print(out.chunks.join(""))
~~~

As with every trait, `write` is not an attribute of the instance: `out.write(x)`
does not exist, and the method is called as `io.Writer.write(out, x)`. Use plain
`read`/`write` methods instead when callers should be able to call them
directly. The trait calls work on any stream, not just on impls:
`io.Reader.read(file, 10)` reads from an `fs` file, and it also runs the plain
`read` method of a user type that has one.
