# cli

The cli module reads a program's command line. A script describes its
arguments the way a call is written, positional names first and then
`name=default`, and gets back an `Args` value with one attribute per name.
Help, usage errors and their exit statuses come with it.

## Common tasks

~~~goblin
import "cli"

# Two arguments and a flag: tool in.txt out.txt --verbose
var args = cli.parse("source", "target", verbose=false)
if args.verbose { print("copying {args.source} to {args.target}") }

# Options with values: the default says what the option holds
var args = cli.parse(
    "url",
    count   = 3,             # --count 5, --count=5         Int
    timeout = 2.5,           # --timeout 10                 Float
    output  = "out.txt",     # --output a.txt               String
    token   = nil,           # --token abc                  String, nil when absent
    include = [],            # --include a --include b      List of String
    dry_run = false,         # --dry-run                    Bool; args.dry_run
)

# Saying more: help text, a short letter, required, choices, a converter
var args = cli.parse(
    cli.Arg("files", "images to resize", many=true),
    width   = cli.Option("target width in pixels", short="w", default=800),
    format  = cli.Option(choices=["png", "jpg"], default="png"),
    output  = cli.Option("directory to write to", short="o", required=true),
    since   = cli.Option("only files newer than this date", convert=time.parse),
    verbose = cli.Flag("say what is being done", short="v", many=true),   # -vv gives 2
    help    = "Resize images into a directory.",
)
for file in args.files { ... }

# An optional argument, and any number of them
var args = cli.parse("pattern", cli.Arg("paths", default=["."], many=true))

# A default from the environment: ordinary code, the default is just a value
var args = cli.parse(port = Int(os.env.get("PORT", default="8080")))

# Subcommands: tool build --release, tool test parser -v
var args = cli.parse(
    build   = cli.Command(release=false, help="Compile the project"),
    test    = cli.Command(cli.Arg("name", default=nil), help="Run the tests"),
    verbose = cli.Flag(short="v"),
    help    = "Project tool.",
)
if args.command == "build" {
    build(release=args.release)
} else {
    test(args.name)
}

# Your own check after parsing: same look, same status
if args.width <= 0 {
    eprint("error: --width must be positive")
    os.exit(2)
}

# Testing: give a List and nothing is printed and nothing exits
func parse(argv=nil) {
    return cli.parse("source", "target", verbose=false, argv=argv)
}
var args = parse()                              # the real command line
print(parse(["a", "b", "--verbose"]).verbose)   # true
try {
    parse(["a"])
} catch e: cli.UsageError {
    print(e.message)                            # missing argument <target>
}
~~~

`docs/examples/cli-tasks.goblin` runs these tasks; `examples/cli.goblin`
covers the rest of the module.

## Members

| Member | Kind | Value |
| --- | --- | --- |
| `parse(*params, help=nil, argv=nil, **options)` | function | Parses a command line against the declaration and returns an `Args` |
| `Arg(name, help=nil, convert=nil, choices=nil, default=…, many=false)` | type | A positional argument |
| `Option(help=nil, short=nil, convert=nil, choices=nil, default=nil, required=false, many=false)` | type | An option that takes a value |
| `Flag(help=nil, short=nil, many=false)` | type | An option that takes no value |
| `Command(*params, help=nil, **options)` | type | A subcommand with its own declaration |
| `Args` | type | What `parse` returns; it cannot be constructed |
| `UsageError` | error kind (parent `Error`) | The command line does not fit the declaration |
| `HelpRequested` | error kind (parent `Error`) | `--help` was given, when parsing a List |

`Arg`, `Option`, `Flag` and `Command` values are immutable descriptions with
no members; one can be stored in a variable and used under several names.
They print as `<Option>` and so on and compare by identity.

## The declaration

`*params`, in order, are the positional arguments. Each is a String (short
for `cli.Arg(name)`) or a `cli.Arg`.

Each keyword in `**options` declares the option `--name`, with `_` written as
`-` on the command line: `dry_run` is `--dry-run` and only that, and the
result attribute is `args.dry_run`. The value is:

| Written | Means | Result attribute |
| --- | --- | --- |
| `false` | `cli.Flag()` | Bool |
| `nil` | `cli.Option()` | String, or `nil` when absent |
| a String, Int or Float | `cli.Option(default=value)` | same type as the default |
| a List | `cli.Option(many=true, default=list)` | List of String |
| `cli.Flag(...)`, `cli.Option(...)` | itself | below |
| `cli.Command(...)` | a subcommand | — |

`help=` is the program's description, shown by `--help`, and `argv=` chooses
the mode (below); neither can be an option name. Names, keywords and `Arg`
names alike, are lower-case letters and digits in words joined by single
`_`s, starting with a letter.

The keywords shared by `Arg`, `Option` and `Flag` mean the same everywhere:

- **`help`**: one line for `--help`. It is the first parameter after the
  name, so it is usually written positionally: `cli.Option("where to write")`.
- **`convert`**: a function from the text to the value: `Int`, `Float`,
  `fs.Path`, `time.parse`, or your own. A ValueError it raises (ParseError
  included) becomes a usage error naming the parameter; any other error
  passes through as the bug it is. Without `convert`, an Int or Float default
  converts with `Int` or `Float`; otherwise the text is kept as a String.
- **`choices`**: a non-empty List; the converted value must `==` one of them.
- **`default`**: the value when the parameter is not given, used as it is:
  not converted and not checked against `choices`.
- **`many`**: the parameter may be given many times. Its value is a List for
  `Arg` and `Option` (a new List on every parse; given values replace the
  default rather than add to it) and the Int count for `Flag`.
- **`short`**: one ASCII letter, giving `-x`. No option has a short letter
  unless it is given one.

An `Arg` is required unless `default=` is given (`default=nil` counts). With
`many=true` it takes all remaining positionals: one or more, or zero or more
when it has a default, which must then be a List. Optional Args come after
required ones, and a `many` Arg is last.

An `Option` is optional unless `required=true`. Absent, it is its `default`
(`nil`; `[]` with `many=true`). Given twice without `many`, the last one
wins. A `Flag` is `false` until given and `true` after, however often; with
`many=true` it counts from `0`.

## Args

An immutable value with exactly one attribute per declared parameter (plus
`command` when subcommands are declared) and no methods, so no option name is
ever taken. `attributes(args)` lists them in declaration order. A wrong name
raises `AttributeError: Args has no attribute 'verbsoe'`, and assigning
raises `AttributeError: Args is read-only`. It prints as
`Args(source="a", target="b", verbose=false)`.

## Two modes

| | `argv=nil` (default) | `argv=` a List of String |
| --- | --- | --- |
| Parses | `os.args` | the List |
| Bad input | writes the error to stderr, exits 2 | raises `cli.UsageError` |
| `--help` | writes the help to stdout, exits 0 | raises `cli.HelpRequested` |
| Program name in texts | base name of `os.program` (`program` when empty, as in the REPL) | same |

The process's command line gets the process's behaviour; a List gets a
function's. Exiting works like `os.exit`: pending deferred calls run first.

- `UsageError`: `message` is the one-line reason, standing alone; the data
  `usage` is the usage line.
- `HelpRequested`: `message` is the complete help text without a trailing
  newline.

A program that reads command lines from somewhere else, a prompt loop for
instance, parses a List and prints `e.message` itself.

## Subcommands

A keyword whose value is a `cli.Command` declares a subcommand, named like an
option is (`dry_run=` is typed `dry-run`). `Command` takes the same
declaration as `parse`, without `argv`.

- With any Command declared, the first positional on the command line names
  the command, and one is required. Top-level positionals cannot be combined
  with commands, and commands do not nest.
- Top-level options are global: accepted before or after the command name. A
  command's own options are accepted only after it.
- The result is flat: `args.command` is the name as typed (`"build"`),
  followed by the global options and the chosen command's parameters. The
  other commands' parameters are not attributes. A command's parameter
  therefore may not reuse a global option's name or be called `command`; two
  commands may use the same name.
- `tool --help` lists the commands; `tool build --help` describes `build`.

## The command line accepted

Tokens are read left to right, and options and positionals may be mixed in
any order.

| Token | Meaning |
| --- | --- |
| `--name` | A Flag; or an Option whose value is the next token |
| `--name=value` | An Option with that value, which may be empty. On a Flag: an error |
| `-x` | A short flag; or a short option whose value is the next token |
| `-abc` | A cluster: `-a -b -c`. The first letter that is an Option takes the rest of the token as its value, or the next token if nothing is left: `-n5`, `-vn 5`, `-vvo out.txt`. A leading `=` in the attached value is dropped (`-o=x` is `x`) |
| `--` | Ends the options: every later token is positional |
| `-` | A positional (conventionally "standard input") |
| `-5`, `-1.5`, `-.5` | A positional or a value: a `-` followed by a digit, or by `.` and a digit, never starts an option |
| anything else | A positional |

The next token is taken as an option's value unless it looks like an option
itself; then the error is `option --output needs a value`. Write a value that
starts with a dash as `--pattern=-x`. Long options are never abbreviated and
names are case-sensitive: `--verb` and `--dry_run` are unknown.

`--help` always asks for help, and `-h` does unless a parameter declared
`short="h"`. It takes effect where the scan reaches it, so an error earlier
on the line is reported instead, while missing parameters are only checked
after the scan: `tool --help` alone always gives help. After `--` it is a
positional. Conversion and `choices` apply as each value is read.

Problems are reported in this order: a wrong declaration, then the first
problem on the line, then a missing command, then missing arguments (all of
them in one message), then the first missing required option.

| Situation | Message |
| --- | --- |
| Unknown option | `unknown option --widht (did you mean --width?)` / `unknown option -x` |
| Option without a value | `option --output needs a value` |
| Flag given a value | `option --verbose takes no value` |
| Int conversion fails | `invalid value "abc" for --count: expected an integer` |
| Float conversion fails | `invalid value "abc" for --timeout: expected a number` |
| Another `convert` raises ValueError | `invalid value "abc" for --since: ` and the error's message |
| Not among `choices` | `invalid value "gif" for --format: expected one of png, jpg` |
| Same, for an argument | `invalid value "abc" for <count>: expected an integer` |
| Too many positionals | `unexpected argument "extra"` |
| Missing positionals | `missing argument <target>` / `missing arguments <source> <target>` |
| Missing required option | `missing required option --output` |
| No command | `missing command (one of build, test)` |
| Unknown command | `unknown command "biuld" (did you mean build?)` |

Options are always named by their long form, whatever was typed. A
suggestion is made when the nearest declared long option (or command) is
within two edits; a tie goes to the earlier declaration.

## Help and errors

The "saying more" declaration above, with the `since` line replaced by
`dry_run = false`, run as `goblin run resize.goblin --help`, writes this to
stdout and exits with status 0:

```text
usage: resize.goblin [options] <files>...

Resize images into a directory.

arguments:
  <files>...              images to resize

options:
  -w, --width <width>     target width in pixels (default: 800)
      --format <png|jpg>  (default: png)
  -o, --output <output>   directory to write to (required)
      --dry-run
  -v, --verbose           say what is being done
  -h, --help              show this help and exit
```

The layout is fixed: no terminal width, no wrapping, no colour.

- The usage line is `usage: `, the program, ` [options]`, then the
  positionals: `<name>` required, `[<name>]` optional, `<name>...` many,
  `[<name>...]` optional many. With commands it is
  `usage: tool.goblin [options] <command> ...`, and for one command
  `usage: tool.goblin build [options] <name>`.
- The `help=` description follows between blank lines, when there is one.
- `arguments:` has a line for each Arg with help, choices or a default; an
  Arg's choices show as `(one of fast, safe)`.
- `commands:` lists each command with its help.
- `options:` lists the options in declaration order, then `--help`; a
  command's page lists its own options, then the global ones. The left column
  is `-x, --name` (four spaces without a short letter), then ` <name>` for an
  Option, `<a|b>` when it has choices, and `...` when it takes many values.
- The help text follows two spaces after the longest left column, then
  ` (default: X)` unless the default is `nil`, `false`, a counting flag's `0`
  or `[]` (a String without quotes, a List as its elements joined by `, `),
  and ` (required)` for a required Option.

A usage error in process mode writes three lines to stderr, nothing to
stdout, and exits with status 2:

```text
error: unknown option --widht (did you mean --width?)
usage: resize.goblin [options] <files>...
Run 'resize.goblin --help' for details.
```

## Wrong declarations

A wrong declaration is the programmer's bug, not the user's: `cli.parse`
raises an ordinary TypeError or ValueError in both modes, before looking at
the command line, so it shows on the first run whatever the arguments.

| Declaration | Message |
| --- | --- |
| An option value of another type (`x={}`) | TypeError `parse() option 'x' must be false, nil, String, Int, Float, List, Flag, Option or Command, got Dict` |
| `x=true` | `parse() option 'x' cannot default to true; declare the opposite flag (no_x=false)` |
| A positional that is not a String or an Arg | TypeError `parse() argument 1 must be String or Arg, got Int` |
| A bad name | `parse() invalid name 'Dry-Run' (use lower case letters, digits and '_')` |
| A name used twice | `parse() duplicate name 'output'` |
| `short` not one ASCII letter | `Option() argument 'short' must be one letter, got "ab"` |
| A short letter used twice | `parse() short option -o is used by 'output' and 'only'` |
| A required Arg after an optional one | `parse() required argument 'target' follows optional argument 'source'` |
| A `many` Arg that is not last | `parse() argument 'files' takes all remaining arguments and must be last` |
| `required=true` with a `default` | `Option() cannot be both required and have a default` |
| `many=true` with a default that is not a List | `Option() argument 'default' must be a List when many is true, got String` |
| A Bool default on an Option | `Option() argument 'default' cannot be a Bool; use cli.Flag()` |
| Empty or non-List `choices` | `Option() argument 'choices' must be a non-empty List` |
| Args together with Commands | `parse() cannot combine arguments with commands` |
| A Command inside a Command | `Command() commands cannot be nested` |
| A command parameter named like a global option, or `command` | `parse() command 'build' reuses the name 'verbose'` |
| `argv` not a List of String | TypeError `parse() argument 'argv' must be a List of String, got String` |

## Left out

- Defaults from environment variables: one line of ordinary code, as in the
  tasks above. Help does not mention the variable.
- `--version`, abbreviated long options, `--no-x` negation, `+x`, single-dash
  words (`-verbose`) and Windows `/x`.
- A placeholder different from the option's name (`--output FILE`): it is
  `<output>`.
- `Args` as a Dict (`to_dict()`, `main(**args)`): `Args` has no methods, so
  no option name is ever taken.
- A `many` Arg before others (`cp <src>... <dst>`): take the last one with
  `args.paths.pop()`.
- Nested subcommands, a default subcommand and command aliases.
- Mutually exclusive or dependent options and value ranges: check after
  parsing, with `eprint` and `os.exit(2)` as above.
- Reading a declaration back, shell completion, wrapping to the terminal
  width, colour and response files (`@args.txt`).
