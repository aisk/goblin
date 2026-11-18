# zip

The zip module reads and writes zip archives. `zip.extract(path, dst)`
unpacks a whole archive in one call, and nothing it writes lands outside
`dst`. To look inside first, `zip.open(path)` gives a Reader: iterate it for
the members, `open(member)` reads one without unpacking the rest, and
`extract(member, dst)` unpacks one. `zip.create(path)` gives a Writer that
packs files, directories and content the program made.

For `.tar` and `.tar.gz` use [tar](./module-tar.md). The two modules share
their vocabulary: the same function and method names, the same `Member` type
and the same `UnsafeError` kind. They differ where the formats differ: a zip
has an index at its end, so its members can be read in any order and the
file must be on disk; a tar is a sequence that can come from a pipe.

## Common tasks

~~~goblin
import "fs"
import "hash"
import "http"
import "json"
import "zip"

# Unpack a downloaded release into a directory
fs.write("goblin-1.4.2.zip", http.get(url).body)
zip.extract("goblin-1.4.2.zip", "tools/goblin")

# Check the download first, then unpack over an earlier version
var download = fs.open("goblin-1.4.2.zip")
defer download.close()
if hash.sha256(download).to_hex() != expected {
    raise ValueError("goblin-1.4.2.zip does not match its checksum")
}
zip.extract("goblin-1.4.2.zip", "tools/goblin", overwrite=true)

# An archive you do not trust: bound what it may write, and say why it was refused
try {
    zip.extract("upload.zip", "incoming", max_size=200 * 1024 * 1024)
} catch e: zip.UnsafeError {
    eprint("upload.zip refused: {e.message}")
    if e.member != nil {
        eprint("offending member: {e.member.name}")
    }
}

# List what a zip contains
var bundle = zip.open("bundle.zip")
defer bundle.close()
for member in bundle {
    var when = member.modified_at.format("%Y-%m-%d %H:%M")
    print("{member.size:>10}  {when}  {member.name}")
}
print(sum(List(bundle).map(func(m) { return m.size })), "bytes when unpacked")

# Read one member without unpacking the rest
var manifest = json.decode(bundle.open("manifest.json"))
var readme = bundle.open("README.md").read().decode()
var notes = bundle.find("NOTES.txt")
if notes != nil {
    for line in bundle.open(notes) {
        print(line)
    }
}

# Unpack only some members
for member in bundle {
    if member.name.has_prefix("docs/") {
        bundle.extract(member, "site")
    }
}

# Pack a directory
var out = zip.create("site.zip")
defer out.close()
out.add("site")                       # stored as site/index.html, site/css/main.css, ...

# The contents of a directory at the top of the archive
var deploy = zip.create("function.zip")
defer deploy.close()
for entry in fs.read_dir("build") {
    deploy.add(entry)                 # stored as handler.js, lib/util.js, ...
}

# Chosen files, under names of your choosing
func unwanted(entry) {
    return entry.name.has_prefix(".") || entry.name == "node_modules"
}
var sources = zip.create("sources.zip")
defer sources.close()
for file in fs.walk("project", skip=unwanted) {
    var inside = file.relative_to("project")
    sources.add(file, name="project-1.4.2/{inside}")
}

# Add content the program made
var results = zip.create("results.zip")
defer results.close()
results.add_data("results.json", json.encode(records))
results.add_data("run.sh", "#!/bin/sh\nexec ./tool --all\n", mode=0o755)
results.add("report.pdf")
~~~

`examples/zip.goblin` runs these tasks on a tree it builds in a temporary
directory.

## Module members

| Member | Returns | Behaviour |
| --- | --- | --- |
| `open(path)` | Reader | Opens an existing zip for reading. Reads its index, but no member content |
| `create(path)` | Writer | Creates the file (mode `0o644`), or replaces its content, and returns a Writer for it. The file is not a valid archive until the Writer's `close()` |
| `extract(path, dst, overwrite=false, max_size=nil)` | `nil` | Unpacks every member under the directory `dst`, which is created with its parents when missing. Opens and closes the archive itself. Every member's name, kind and declared size is checked before anything is written ([What extract does](#what-extract-does-and-what-it-refuses)). `max_size` is the most bytes the whole extraction may write; `nil` is no limit |
| `UnsafeError` | error kind | Raised by `extract`, the function and the method, for a member it refuses. Its parent is `Error`. It is the same kind as `tar.UnsafeError` |
| `Reader`, `Writer`, `Member` | type names | For `type_of`. Calling `Reader` raises `TypeError: Reader cannot be constructed; use zip.open(path)`, and `Writer` names `zip.create(path)` |

`path` and `dst` are a String or an `fs.Path`.

## Reader

An open zip, from `zip.open`. It holds the file open until `close()`. It
shows as `<zip.Reader bundle.zip>`. A Reader is not meant to be shared
between goblins.

| Member | Kind | Value |
| --- | --- | --- |
| `path` | property | The Path it was opened with |
| `closed` | property | Bool |
| `for member in reader` | iteration | Every Member, in the order the archive lists them. Each loop starts from the first, so `List(reader)` is the whole listing. Reads nothing from disk |
| `find(name)` | method | The Member stored under exactly `name` (a String), or `nil`. When a name is stored twice, the last one, which is the one unpacking tools keep |
| `open(member)` | method | An `fs.File` open for reading the member's content, decompressed as it is read. `member` is a Member of this Reader or a name |
| `extract(member, dst, name=nil, overwrite=false)` | method | Writes that one member under the directory `dst` and returns the Path written. It lands at its own name, or at `name` (a String or Path, relative to `dst`) when given. The checks are those of the module function, applied to the name it lands at. Creates `dst` and missing parent directories |
| `close()` | method | Releases the file and returns `nil`. Calling it again does nothing. Member Files from this Reader stop working |

There is no `read` method on the archive: a method of that name would make
the Reader look like a readable to `fs.write` and `json.decode`.
`reader.open(member).read()` is a member's whole content.

### Member Files

The File from `open(member)`:

- reads the member's content; its `path` is `nil` and it shows as
  `<File bundle.zip:manifest.json>`;
- is a readable, so `json.decode`, `csv.decode_dicts`, `hash.sha256` and
  `fs.write(path, file)` take it; `read()` gives Bytes and `for line in file`
  gives lines;
- holds no resource of its own: `close()` on it is allowed and never needed,
  and the Reader's `close()` is what releases the file on disk;
- stops working when its Reader is closed:
  `ValueError: read() member "a.log" is no longer open`;
- raises ParseError `read() corrupt data in member "x" of "bundle.zip"` from
  the call that reaches damage, including a failed checksum at the end of the
  member.

## Writer

A zip being written, from `zip.create`. It shows as `<zip.Writer site.zip>`.
A Writer is not meant to be shared between goblins.

| Member | Kind | Value |
| --- | --- | --- |
| `path` | property | The Path it was created with |
| `closed` | property | Bool |
| `add(path, name=nil)` | method | Stores what `path` names on disk and returns `nil`. A file: its content (compressed and streamed, never held whole), permission bits and modification time. A directory: an entry for it and everything below it, names sorted within each directory. A symbolic link: stored as a link, never followed. It is stored under `name` (a String or Path), which defaults to the last component of `path`; entries below a directory get `name` in front of their relative path. The archive's own file is passed over when it lies inside an added directory |
| `add_data(name, data, mode=0o644)` | method | Stores `data` (a String, Bytes, or a readable, which is streamed) as a file member called `name`, with permission bits `mode` and the current time; returns `nil` |
| `close()` | method | Finishes the archive by writing its index, then releases the file; returns `nil`. Calling it again does nothing |

Until `close()` the file is not a zip, so a Writer always gets
`defer archive.close()`.

The name of the last component is the default because the alternative, the
path as given, would put `build/` in front of every name for
`add("build/site")` and would store an absolute path as `home/me/…`. The
default stores `site/…`, as `cp -r build/site dst/` would.

**Names given to a Writer.** A name is written the way a relative path is
written on this platform and is stored with `/`. It must be usable when
unpacked: not empty, not absolute, with no `..` component and no NUL or `\`.
Anything else raises `ValueError: add() name "../x" would be unpacked outside
its directory`. A Writer therefore cannot produce an archive that `extract`
refuses for its names.

Every file is compressed with DEFLATE at the usual level. Names that are not
ASCII are stored as UTF-8 and flagged as such.

## Member

One type shared with tar: `zip.Member == tar.Member`. A Member is an
immutable description of one entry, with no I/O; its content comes from the
archive through `open(member)`. It shows as `<Member docs/guide.md>`. It
cannot be constructed (`TypeError: Member cannot be constructed; iterate an
archive from zip.open(path) or tar.open(path)`), and two Members are equal
only when they are the same value.

| Property | Value |
| --- | --- |
| `name` | String: the name as stored, components separated by `/`, without the trailing `/` of a directory entry. It is not cleaned: `../x` and `/etc/passwd` are reported as they are. Bytes that are not valid UTF-8 are shown as U+FFFD |
| `size` | Int: bytes of content when unpacked; `0` for a directory or a link |
| `modified_at` | `time.Time` |
| `mode` | Int: permission bits as stored (`member.mode == 0o755`). A zip written on a system without them reports `0o644` for a file and `0o755` for a directory |
| `is_file` | Bool: a regular file |
| `is_dir` | Bool |
| `is_symlink` | Bool: a symbolic link |
| `link_target` | String: what a link points to, as stored; `nil` unless the member is a link |

The names and meanings are those of `fs.FileInfo`, plus `name` and
`link_target`. A member that is none of file, directory or link (a device, a
FIFO) has all three `is_` properties false.

A zip stores wall-clock time without a zone unless the program that made it
added the UTC timestamp field. Without that field the stored time is read as
UTC.

## What extract does, and what it refuses

The same rules hold for `zip.extract`, `tar.extract` and the `extract` method
of both Readers. "The name" is the member's `name`, or the `name=` argument
when the method was given one.

### What is created

| Member | Result under `dst` |
| --- | --- |
| File | A file with the member's content, its permission bits and its modification time. Missing parent directories are created (`0o755`), since many archives have no entries for them |
| Directory | The directory. An existing one is used as it is |
| Symbolic link | The link, when its target is relative and, followed from where the link lands, stays inside `dst`. Otherwise it is refused |
| Hard link (tar) | A hard link to the file already unpacked under the target name. The target is checked as a member name is |
| Anything else (device, FIFO, socket) | Refused |

Permission bits are the stored ones, limited to `0o777` and then by the
process's umask, as for any file a program creates. Set-user-id, set-group-id
and sticky bits are never applied. A directory always gets read, write and
search permission for its owner, so that unpacking can continue inside it.
Ownership is never applied: everything belongs to the user running the
script. Directory modification times are not restored.

### What is refused

| Hazard | Default | Reported as | To do it anyway |
| --- | --- | --- | --- |
| A name that is absolute (`/etc/passwd`, `C:\x`), has a `..` component, contains `\` or NUL, is empty, or is a reserved device name on Windows | Refused | `UnsafeError: extract() member "../../etc/passwd" of "upload.zip" would be written outside "incoming"` | The method, with a `name=` the script computed |
| A symbolic link whose target is absolute or leads outside `dst` | Refused | `UnsafeError: extract() link "current" of "x.tar" points outside "app": "/etc"` | Not possible with this module; skip the member in a loop |
| A name that passes through a symbolic link leading outside `dst`, whether the link was in `dst` before or came from this archive | Refused by the operating system interface used, whatever the name looked like | `UnsafeError: extract() member "lib/x" of "x.tar" would be written outside "app"` | — |
| A device, FIFO or socket | Refused | `UnsafeError: extract() member "dev/null" of "rootfs.tar" is a device` | Skip the member in a loop |
| More content than the member declares (a header that understates its size) | Stops at the declared size; nothing is left at the name | `UnsafeError: extract() member "a.bin" of "x.zip" is larger than the 1024 bytes it declares` | — |
| The members' declared sizes add up to more than `max_size` (module function only) | No limit unless `max_size` is given | `UnsafeError: extract() "upload.zip" unpacks to 6442450944 bytes, over max_size 209715200` | A larger `max_size`, or `nil` |
| A file already at the name | Refused | `ExistError: extract() "app/bin/tool" already exists (pass overwrite=true)`, with data `path` | `overwrite=true` replaces files and links. The existing entry is removed and a new one created, so nothing is ever written through an existing link |
| A directory where a file must go, or a file where a directory must go | Refused, with or without `overwrite` | `ExistError: extract() "app/bin" is a directory` (or `is not a directory`), with data `path` | `fs.remove` it first |
| The same name twice in one archive | The second is "a file already at the name" | ExistError as above | `overwrite=true`, which keeps the last, as `tar` and `unzip` do |

When the method was given a `name=` that is refused, the message adds where
the member would have landed:
`extract() member "a.txt" of "x.zip" would be written outside "dst" as "../a.txt"`.

These rules follow from the table:

- **Nothing is skipped and no name is changed.** An archive either unpacks as
  it says or raises. `tar(1)` strips a leading `/` and carries on, which
  unpacks something other than what the archive says without telling the
  caller.
- **`zip.extract` checks every member's name, kind and declared size before
  it writes anything**, because a zip's index is available up front. A zip
  refused for those reasons leaves `dst` untouched. Link targets are stored as
  content and are checked when they are reached.
- **`tar.extract` stops at the member it refuses.** The members before it
  stay unpacked: a tar cannot be checked ahead without reading all of it. A
  script that must not leave a partial tree unpacks into `fs.mkdir_temp()`
  and moves the result.
- **The default does not protect against an archive that is honestly huge**,
  or one made to be (a "zip bomb" declares its true, enormous sizes). Content
  is streamed, so memory is never at risk; disk space is. For input from a
  source the script does not trust, pass `max_size`. With `zip` the refusal
  comes before anything is written; with `tar` it comes at the member whose
  declared size crosses the limit, before that member is written.
- **The method takes no `max_size`.** In a loop the script has `member.size`,
  and no member can write more than that.
- Reading (`for member in`, `open(member)`) never refuses a name. A listing
  shows `../../etc/passwd` as it is stored. Only `extract` writes to disk
  under a member's name.

There is no default size limit because any number would be a policy that
refuses some ordinary large archives, and changing it later would break
scripts either way. What the default does guarantee is where files land and
that no member exceeds its declared size.

## Errors

| From | Kinds (besides TypeError and ValueError for arguments) | Data |
| --- | --- | --- |
| `open`, `create`, `extract`: the archive file | NotExistError, PermissionError, IOError | `path`: the archive |
| `open`, a member File, `extract`: not a zip, damaged, encrypted, unsupported compression | ParseError: `open() "x.zip" is not a zip archive`, `open() member "x" of "bundle.zip" is encrypted`, `open() member "x" of "bundle.zip" uses a compression method zip does not read`, `read() corrupt data in member "x" of "bundle.zip"` | `path`: the archive |
| `Reader.open(name)`, `Reader.extract(name, …)`: no such member | KeyError `open() no member "x" in "bundle.zip"` | — |
| `extract`: a refused member | UnsafeError | `member`: the Member refused, `nil` when the refusal is about the whole archive (`max_size`). `path`: the archive |
| `extract`: something in the way under `dst`, or `dst` cannot be written | ExistError, PermissionError, IOError | `path`: the path under `dst` |
| `add`: the file being added | NotExistError, PermissionError, IOError | `path`: that file |
| A closed Reader or Writer; a Member of another archive; `open` of a directory or link; a bad name given to a Writer; a name already in the archive; `add(".")` without `name=`; a socket or device given to `add`; `mode` outside `0`–`0o777` | ValueError | — |

UnsafeError is not an IOError, because the disk and the archive both worked:
a handler that receives archives from elsewhere catches exactly it, and can
tell it from a full disk (IOError) and from a damaged archive (ParseError).

## Left out

| Left out | Instead |
| --- | --- |
| Encrypted zips, reading or writing | `exec.run(["unzip", "-P", password, file])`. Go's library has no zip encryption; the legacy scheme is broken and AES is an extension. `open(path, password=)` can be added later |
| Changing an existing archive: appending, deleting or replacing a member | Write a new archive |
| Copying a member into another archive without unpacking it | `extract` and `add`, or for small members `add_data(member.name, source.open(member).read())` |
| A one-call "pack this directory" | Three lines: `create`, `defer close()`, `add`. They show what the names inside will be |
| A one-call listing | `open`, `defer close()`, a loop |
| A Reader over Bytes or a stream; a Writer over a writable | `fs.write(file, data)` first: reading needs a seekable file because the index is at the end. The names `zip.Reader(source)` and `zip.Writer(target)` stay free for this |
| A compression level, or storing without compression | None; `level=` on `create` can be added later |
| Setting a member's modification time; an empty directory or a link from `add_data` | `add` takes times and kinds from disk |
| `exclude=` or `skip=` on `add` and `extract` | A loop over `fs.walk(path, skip=…)` with `add`; a loop with `extract(member, dst)` |
| `strip=` on `extract` | A loop with `name=`, as the [tar chapter](./module-tar.md#common-tasks) shows for a source tarball |
| Progress reporting | A loop with the method; `member.size` gives the total up front |
| Names in a legacy encoding (CP437, Shift-JIS) | They are shown with U+FFFD; unpack such a member with `name=` |
| Restoring ownership, set-id bits and directory times; links that point outside | Not offered: these are the unsafe parts |
