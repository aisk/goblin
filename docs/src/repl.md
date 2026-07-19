# Using the REPL

Start a persistent interactive Goblin session with:

~~~sh
$ goblin repl
Goblin REPL. Press Ctrl-D to exit.
>>>
~~~

Expressions print their value automatically; declarations and statements do
not. Names, imports, functions, and types stay available until the session
exits.

~~~text
>>> 1 + 2 * 3
7
>>> var values = [1, 2, 3]
>>> values.push(4)
>>> values
[1, 2, 3, 4]
>>> import "math"
>>> math.sqrt(81)
9
~~~

Relative imports are resolved from the directory where the REPL starts, so
start it from a project directory when testing a local module.

## Multi-line input

When parentheses, brackets, or braces are unbalanced, the prompt changes to
`...` and the REPL keeps collecting lines. This is how to enter functions,
types, loops, and try/catch blocks.

~~~text
>>> func square(x) {
...     return x * x
... }
>>> square(12)
144
~~~

Enter a blank line to force evaluation of a malformed fragment, or press
Ctrl-C to discard the input currently being collected.

## Completion and history

Press Tab to complete visible names, keywords, and member paths. For example,
after `import "json"`, type `json.` and press Tab to see members. Completion
only reads attributes; it does not call Goblin functions.

History is stored in `~/.goblin_history`; use Up/Down to revisit entries and
Ctrl-D to exit. Use the REPL for small experiments and API discovery, then move
repeatable work into a `.goblin` file.
