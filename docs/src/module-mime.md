# mime

The mime module maps filename extensions and MIME types. It is useful when
constructing HTTP Content-Type headers or categorizing uploaded files.

~~~goblin
import "mime"

print(mime.type_by_extension(".json"))
print(mime.extensions_by_type("application/json"))
~~~

type_by_extension(extension) returns a string, or an empty string when the
extension is unknown. Include the leading dot in the extension.

extensions_by_type(type) returns a list of known extensions. It can raise
ParseError when the supplied MIME type is invalid.
