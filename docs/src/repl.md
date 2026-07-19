# Using the REPL

The REPL is a persistent interactive Goblin session. Start it with:

~~~sh
$ goblin repl
Goblin REPL. Press Ctrl-D to exit.
>>>
~~~

Use it to test expressions, inspect values, and try an API before putting code
in a source file. The REPL prints the value of an expression automatically; it
stays quiet after declarations and statements.

~~~text
>>> 1 + 2 * 3
7
>>> var language = "Goblin"
>>> language.upper()
GOBLIN
~~~

## Persistent session state

Names, imports, functions, and types remain available until the session exits.
This lets you build a small experiment one step at a time.

~~~text
>>> var values = [1, 2, 3]
>>> values.push(4)
>>> values
[1, 2, 3, 4]
>>> import "math"
>>> math.sqrt(81)
9
~~~

Relative imports are resolved from the directory in which you start the REPL.
Start it from a project directory when testing local modules.

## Multi-line input

When parentheses, brackets, or braces are not yet balanced, the REPL changes
the prompt to ... and keeps collecting lines. This is how to enter functions,
types, loops, and try/catch blocks.

~~~text
>>> func square(x) {
...     return x * x
... }
>>> square(12)
144
~~~

Enter a blank line after a malformed multi-line fragment to force evaluation
and see its error. Press Ctrl-C to discard the input currently being collected
and return to a fresh prompt.

## Completion and history

Press Tab to complete visible names, keywords, and safe member paths. For
example, after importing json, type json. and press Tab to see its members.
Completion only reads attributes; it does not call Goblin functions.

The REPL stores command history in ~/.goblin_history. Use the usual terminal
line-editing keys and Up/Down arrows to revisit previous entries. Press Ctrl-D
to exit cleanly.

## When to use the REPL

Use the REPL for short experiments and API discovery. Move repeatable work into
a .goblin file and run it with goblin run. Build an executable only after the
program is behaving as intended.
