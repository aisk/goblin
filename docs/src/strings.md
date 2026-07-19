# Strings

Strings are immutable Unicode text values written with double quotes. Escape a
double quote or a backslash with a backslash.

~~~goblin
var message = "say: \"hello\""
print(message)        # say: "hello"
print(message.size()) # 12
~~~

size() counts Unicode characters, not bytes. Indexing and iteration also
operate on characters.

~~~goblin
var language = "Goblin"
print(language[0]) # G

for character in language {
    print(character)
}
~~~

An invalid index raises IndexError.

## Combining and converting text

Use + to concatenate strings. A string can also concatenate an integer or
boolean, and * repeats it by an integer count.

~~~goblin
print("go" + "blin") # goblin
print("item-" + 3)   # item-3
print("ha" * 3)      # hahaha
~~~

Str(value) converts any value to its display text. This is useful when
building a string from a float, list, custom type, or nil.

~~~goblin
var label = "port=" + Str(8080)
print(label)
~~~

## Common string methods

| Method | Purpose |
| --- | --- |
| size() | Character count |
| upper() / lower() / title() | Change letter case |
| contains(substring) | Test for a substring |
| has_prefix(prefix) / has_suffix(suffix) | Test the beginning or end |
| index(substring) / last_index(substring) | Find a substring; returns -1 if absent |
| count(substring) | Count non-overlapping occurrences |
| replace(old, new, count=-1) | Replace all occurrences by default |
| split(separator, count=-1) | Split into a list |
| split_after(separator, count=-1) | Split while retaining separators |
| trim(cutset=nil) | Trim Unicode whitespace or supplied characters |
| trim_prefix(prefix) / trim_suffix(suffix) | Remove one matching edge |
| cut(separator) | Split once, returning a three-element result |
| repeat(count) | Repeat text |

Methods accept named arguments as well as positional ones.

~~~goblin
var title = "  Goblin book  "
print(title.trim().upper())
print("a,b,c".split(sep=",", count=2))
print("one one one".replace(old="one", new="two", count=2))
print("config.toml".trim_suffix(".toml"))
~~~

trim() with no argument removes Unicode whitespace. Supplying a string trims
any of its characters from both ends; it does not remove an exact substring.
Use trim_prefix() or trim_suffix() when that distinction matters.

## Common text-processing patterns

Use trim() before validating user-supplied text, split() to turn a delimited
setting into a list, and replace() when normalizing a known spelling or format.

~~~goblin
var raw_tags = " go, language,tools "
var tags = []
for tag in raw_tags.split(",") {
    tags.push(tag.trim())
}
print(tags)

var filename = "report.TXT"
if filename.lower().has_suffix(".txt") {
    print("text file")
}
~~~

Use contains() when a yes/no answer is enough. Use index() or last_index() when
you need the position, such as separating a filename from its final extension.
