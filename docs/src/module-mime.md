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

## Using MIME information with files and HTTP

Pass a suffix, including its leading dot, to type_by_extension(). The returned
type may include a charset parameter and is empty when no mapping is known.

~~~goblin
var filename = "report.json"
var content_type = mime.type_by_extension(".json")
if content_type == "" {
    content_type = "application/octet-stream"
}
print(content_type)
~~~

extensions_by_type() is useful when a program accepts a declared content type
and needs to show or validate the associated filename suffixes.

~~~goblin
var image_extensions = mime.extensions_by_type("image/png")
print(image_extensions)
~~~

MIME lookup only identifies a probable media type. Do not use a filename
extension alone as a security check for untrusted content.
