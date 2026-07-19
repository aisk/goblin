# Scope and declarations

`var` creates a name in the current lexical scope. A name declared in a block
is visible inside nested blocks, but not after the block ends. Assignment
updates the nearest existing name; use `var` when a new local binding is
intended.

~~~goblin
var message = "outside"
if true {
    var detail = "inside"
    message = "changed"
    print(detail)
}
print(message) # changed
# detail is not visible here
~~~

Functions capture the surrounding lexical scope. Each `for` iteration has its
own loop binding, so functions created in a loop retain that iteration's value.

~~~goblin
var readers = []
for value in [1, 2, 3] {
    readers.push(func() { return value })
}
print(readers[0](), readers[1](), readers[2]()) # 1 2 3
~~~

## Declarations at module scope

`import`, `type`, and `export` are allowed only at module scope. A named
function can call itself recursively, but ordinary uses of a top-level name
must appear after its declaration.

~~~goblin
func double(value) {
    return value * 2
}

print(double(21))
~~~

Names must be declared before ordinary expressions use them. In particular,
local `var` declarations and top-level functions are not available before
their declaration. Keep declarations near the beginning of the block when that
makes a function easier to read.
