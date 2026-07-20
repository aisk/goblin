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

`import`, `type`, and `export` are allowed only at module scope. Module-level
`import`, `func`, and `type` names are hoisted: they are visible throughout
the module regardless of where the definition appears, so functions may call
functions defined later — including mutually recursive pairs.

~~~goblin
func is_even(n) {
    if n == 0 { return true }
    return is_odd(n - 1)
}

func is_odd(n) {
    if n == 0 { return false }
    return is_even(n - 1)
}

print(is_even(10))
~~~

Other names must be declared before use: local `var` declarations (and nested
function definitions) are not available before their declaration. Keep
declarations near the beginning of the block when that makes a function easier
to read.
