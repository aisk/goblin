# tar

The tar module reads and writes tar archives, plain or gzip-compressed:
`.tar` and `.tar.gz` files, and tar data arriving from a download or another
program. `tar.extract(path, dst)` unpacks a whole archive in one call, and
nothing it writes lands outside `dst`. A tar is a sequence of members, so a
Reader visits them in order, once, and reads only the member it is at: an
archive larger than memory is fine. `tar.create(path)` gives a Writer that
packs files, directories and content the program made.

For `.zip` use [zip](./module-zip.md). The two modules share their
vocabulary: the same function and method names, the same `Member` type and
the same `UnsafeError` kind. They differ where the formats differ: a zip has
an index at its end, so its members can be read in any order and the file
must be on disk; a tar can come from a pipe, but only in order.

## Common tasks

~~~goblin
import "exec"
import "fs"
import "gzip"
import "hash"
import "http"
import "tar"

# Unpack a downloaded release into a directory
fs.write("goblin-1.4.2.tar.gz", http.get(url).body)
tar.extract("goblin-1.4.2.tar.gz", "tools/goblin")

# Check the download first, then unpack over an earlier version
var download = fs.open("goblin-1.4.2.tar.gz")
defer download.close()
if hash.sha256(download).to_hex() != expected {
    raise ValueError("goblin-1.4.2.tar.gz does not match its checksum")
}
tar.extract("goblin-1.4.2.tar.gz", "tools/goblin", overwrite=true)

# An archive you do not trust: bound what it may write, and say why it was refused
try {
    tar.extract("upload.tar.gz", "incoming", max_size=200 * 1024 * 1024)
} catch e: tar.UnsafeError {
    eprint("upload.tar.gz refused: {e.message}")
    eprint("offending member: {e.member.name}")
}

# All or nothing: unpack beside the target, then move into place
var staging = fs.mkdir_temp(prefix="unpack-")
defer fs.remove(staging, recursive=true)
tar.extract("release.tar.gz", staging.join("app"))
fs.move(staging.join("app"), "app")

# Go through a tar of any size, one member at a time
var logs = tar.open("logs-2026-11.tar.gz")
defer logs.close()
var errors = 0
for member in logs {
    if member.is_file && member.name.has_suffix(".log") {
        for line in logs.open(member) {
            if line.contains("ERROR") {
                errors = errors + 1
            }
        }
    }
}

# Read one member: stop when it has been found
var source = tar.open("project.tar.gz")
defer source.close()
var version = nil
for member in source {
    if member.name == "project/VERSION" {
        version = source.open(member).read().decode().trim()
        break
    }
}

# Unpack a source tarball without its top-level directory
var release = tar.open("project-1.4.2.tar.gz")
defer release.close()
for member in release {
    var top, rest = member.name.cut("/")
    if rest != nil && rest != "" {
        release.extract(member, "project", name=rest)
    }
}

# Unpack a download as it arrives, without saving the archive
var response = http.get(url, timeout=nil)
defer response.body.close()
var stream = tar.Reader(response.body)
for member in stream {
    stream.extract(member, "data")
}

# Pack a directory into a .tar.gz
var backup = tar.create("site.tar.gz", gzip=true)
defer backup.close()
backup.add("site")                    # stored as site/index.html, site/css/main.css, ...

# Chosen files, under names of your choosing
func unwanted(entry) {
    return entry.name.has_prefix(".") || entry.name == "node_modules"
}
var sources = tar.create("sources.tar.gz", gzip=true)
defer sources.close()
for file in fs.walk("project", skip=unwanted) {
    var inside = file.relative_to("project")
    sources.add(file, name="project-1.4.2/{inside}")
}

# The smallest .tar.gz: tar inside the File that gzip.open gives
var packed = gzip.open("corpus.tar.gz", write=true, level=9)
defer packed.close()
var corpus = tar.Writer(packed)
defer corpus.close()                  # runs first: ends the tar data, then packed.close() ends the gzip data
corpus.add("corpus")

# A compression tar does not read: let the program do it
var xz = exec.start(["xz", "-dc", "rootfs.tar.xz"], stdout=exec.PIPE)
var rootfs = tar.Reader(xz.stdout)
for member in rootfs {
    print(member.name)
}
xz.wait()
~~~

`examples/tar.goblin` runs these tasks on a tree it builds in a temporary
directory.

## Module members

| Member | Returns | Behaviour |
| --- | --- | --- |
| `open(path)` | Reader | Opens an existing tar for reading, gzip-compressed or not. Gzip is recognised by the first bytes of the file, not by its name. Members are read only when the Reader is iterated |
| `create(path, gzip=false)` | Writer | Creates the file (mode `0o644`), or replaces its content. `gzip=true` compresses the archive as it is written. The file's name decides nothing |
| `extract(path, dst, overwrite=false, max_size=nil)` | `nil` | Unpacks every member, gzip-compressed or not, under the directory `dst`, which is created with its parents when missing. Opens and closes the archive itself. It stops at the first member it refuses ([What extract does](#what-extract-does-and-what-it-refuses)). `max_size` is the most bytes the whole extraction may write; `nil` is no limit |
| `Reader(source)` | Reader | A Reader over a readable the caller owns: a File, a response `body`, a process's `stdout`. Gzip is recognised the same way. `close()` on this Reader does not close `source` |
| `Writer(target)` | Writer | A Writer over a writable the caller owns: a File from `fs.open(path, write=true)` or `gzip.open(path, write=true)`, a process's `stdin`. It writes plain tar. `close()` finishes the tar data and does not close `target` |
| `UnsafeError` | error kind | Raised by `extract`, the function and the method, for a member it refuses. Its parent is `Error`. It is the same kind as `zip.UnsafeError` |
| `Member` | type name | [Member](#member) |

`path` and `dst` are a String or an `fs.Path`.

Reading recognises gzip by content because a script does not choose what it
is given, and nearly every tar a script meets is compressed; it needs no rule
about file names. Writing is the script's choice, so it says so with
`gzip=true`, or with `tar.Writer` over `gzip.open(path, write=true, level=9)`
when it wants a compression level. Another compression goes through an
`exec` pipe, as the last task above shows.

## Reader

A tar being read, from `tar.open` or `tar.Reader`. It is a stream of members
in the way a File is a stream of lines. It shows as `<tar.Reader
logs.tar.gz>`, or `<tar.Reader>` over a stream. A Reader is not meant to be
shared between goblins.

| Member | Kind | Value |
| --- | --- | --- |
| `path` | property | The Path it was opened with; `nil` over a stream |
| `closed` | property | Bool |
| `for member in reader` | iteration | The **remaining** Members, one per step. Each step moves to the next member and passes over whatever content of the previous one was not read. The loop does not close the Reader, and a loop left with `break` leaves it at that member: a later loop continues after it. It never holds more than one member's header |
| `open(member)` | method | An `fs.File` open for reading the content of the **current** member: the one the loop most recently gave. Each call gives a File over the same content, continuing where the last one stopped |
| `extract(member, dst, name=nil, overwrite=false)` | method | Writes the current member under the directory `dst` and returns the Path written. It lands at its own name, or at `name` (a String or Path, relative to `dst`) when given. The checks are those of the module function, applied to the name it lands at. Creates `dst` and missing parent directories |
| `close()` | method | Releases the file when the Reader came from `tar.open` and returns `nil`. Calling it again does nothing |

There is no `find`, and `open` takes no name: finding a member means reading
past the ones before it, and that belongs in a loop the reader of the script
can see.

A File from `open(member)` reads ahead when it is read line by line, so a
second File over the same member continues after what the first one
buffered. `extract` of a member whose content was already read, in part or
in full, through `open` or by an earlier `extract`, raises
`ValueError: extract() member "a.log" was already read; a tar archive is
read in order`, because what is left is not the member.

### Member Files

The File from `open(member)`:

- reads the member's content; its `path` is `nil` and it shows as
  `<File logs.tar.gz:a.log>` (`<File a.log>` over a stream);
- is a readable, so `json.decode`, `csv.decode_dicts`, `hash.sha256` and
  `fs.write(path, file)` take it; `read()` gives Bytes and `for line in file`
  gives lines;
- holds no resource of its own: `close()` on it is allowed and never needed;
- stops working when the loop moves to the next member, and when its Reader
  is closed: `ValueError: read() member "a.log" is no longer open`, and from
  then on its `closed` is `true`;
- raises ParseError `read() corrupt data in member "a.log" of "logs.tar.gz"`
  from the call that reaches damage.

## Writer

A tar being written, from `tar.create` or `tar.Writer`. It shows as
`<tar.Writer site.tar.gz>`, or `<tar.Writer>` over a stream. A Writer is not
meant to be shared between goblins.

| Member | Kind | Value |
| --- | --- | --- |
| `path` | property | The Path it was created with; `nil` over a stream |
| `closed` | property | Bool |
| `add(path, name=nil)` | method | Stores what `path` names on disk and returns `nil`. A file: its content, permission bits and modification time. A directory: an entry for it and everything below it, names sorted within each directory. A symbolic link: stored as a link, never followed. It is stored under `name` (a String or Path), which defaults to the last component of `path`; entries below a directory get `name` in front of their relative path. The archive's own file is passed over when it lies inside an added directory. The owner's and group's ids and names are not stored (ids are written as 0, names empty), so an archive does not record who made it and is the same on every machine |
| `add_data(name, data, mode=0o644)` | method | Stores `data`, a String or Bytes, as a file member called `name`, with permission bits `mode` and the current time; returns `nil`. A tar member's size is written before its content, so a stream of unknown length cannot be stored without holding it: content too big for memory goes to a file first and is added with `add` |
| `close()` | method | Writes the end-of-archive marker, finishes the gzip data when the Writer came from `create(path, gzip=true)`, and releases the file when it came from `create`; returns `nil`. Over a stream it does not close `target`. Calling it again does nothing |

Until `close()` the archive is incomplete, so a Writer always gets
`defer archive.close()`. With `gzip.open` underneath, the tar Writer is closed
first and the File second; two `defer`s in the order of creation do exactly
that.

The name of the last component is the default because the alternative, the
path as given, would put `build/` in front of every name for
`add("build/site")` and would store an absolute path as `home/me/…`. The
default stores `site/…`, as `cp -r build/site dst/` would.

**Names given to a Writer.** A name is written the way a relative path is
written on this platform and is stored with `/`. It must be usable when
unpacked: not empty, not absolute, with no `..` component and no NUL or `\`.
Anything else raises `ValueError: add() name "../x" would be unpacked outside
its directory`; an empty String raises `ValueError: add() argument 'name' must
not be empty`, as an empty path does everywhere. A Writer therefore cannot produce an archive that `extract`
refuses for its names.

The format written is the common one (ustar), with the extended (pax) header
only for a member that needs it, such as a name over 100 bytes or a size over
8 GiB. There is no setting.

## Member

One type shared with zip: `tar.Member == zip.Member`. A Member is an
immutable description of one entry, with no I/O; its content comes from the
archive through `open(member)`. It shows as `<Member docs/guide.md>`. It
cannot be constructed (`TypeError: Member cannot be constructed; iterate an
archive from zip.open(path) or tar.open(path)`), and two Members are equal
only when they are the same value.

| Property | Value |
| --- | --- |
| `name` | String: the name as stored, components separated by `/`, without the trailing `/` of a directory entry. It is not cleaned: `../x` and `/etc/passwd` are reported as they are. Bytes that are not valid UTF-8 are shown as U+FFFD |
| `size` | Int: bytes of content when unpacked; `0` for a directory or a link |
| `modified_at` | `time.Time`, in the local zone (as `fs.stat` gives it) |
| `mode` | Int: permission bits as stored (`member.mode == 0o755`). |
| `is_file` | Bool: a regular file |
| `is_dir` | Bool |
| `is_symlink` | Bool: a symbolic link |
| `link_target` | String: what a link points to, as stored; `nil` unless the member is a link |

The names and meanings are those of `fs.FileInfo`, plus `name` and
`link_target`. A member that is none of file, directory or link (a device, a
FIFO) has all three `is_` properties false.

A tar hard link has `link_target` set and `is_file`, `is_dir` and
`is_symlink` all false; no other member has that combination.

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
| `open`, `Reader`: a compression tar does not read | ParseError `open() "x.tar.xz" is xz-compressed; tar reads plain and gzip-compressed archives` (also bzip2 and zstd) | `path`: the archive, `nil` over a stream |
| Iteration, a member File, `extract`: not a tar, damaged | ParseError: `"x.tar" is not a tar archive` at the first step, `corrupt tar data in "x.tar"` later, `read() corrupt data in member "a.log" of "x.tar"`; `extract() "x.tar" is not a tar archive` and `extract() corrupt tar data in "x.tar"` from the module function | `path`: the archive, `nil` over a stream |
| `extract`: a refused member | UnsafeError | `member`: the Member refused. `path`: the archive, `nil` over a stream |
| `extract`: something in the way under `dst`, or `dst` cannot be written | ExistError, PermissionError, IOError | `path`: the path under `dst` |
| `add`: the file being added | NotExistError, PermissionError, IOError | `path`: that file |
| `Reader(source)` without `read(size)`, `Writer(target)` without `write(data)` | TypeError `Reader() argument 'source' must have a read(size) method, got String` | — |
| `open` or `extract` of a Member that is not the current one | ValueError `open() "a.log" is not the current member; a tar archive is read in order` | — |
| `open(name)` with a String | TypeError `open() argument 'member' must be Member, got String` | — |
| `add_data` with a readable | TypeError `add_data() argument 'data' must be String or Bytes, got File (a tar header holds the size before the content; write a stream to a file and add() it)` | — |
| A closed Reader or Writer; `open` of a directory or link; a bad name given to a Writer; a name already in the archive; `add(".")` without `name=`; a socket or device given to `add`; `mode` outside `0`–`0o777` | ValueError | — |

UnsafeError is not an IOError, because the disk and the archive both worked:
a handler that receives archives from elsewhere catches exactly it, and can
tell it from a full disk (IOError) and from a damaged archive (ParseError).

Over a stream there is no archive to name, so messages leave out
`of "x.tar"`: `extract() member "../x" would be written outside "data"`.

## Left out

| Left out | Instead |
| --- | --- |
| Reading `.tar.xz`, `.tar.zst` or `.tar.bz2` in the module | `tar.Reader` over an `exec` pipe, as in the tasks above. The error names the compression it found, and recognition by content can grow a format without an API change |
| Changing an existing archive: appending, deleting or replacing a member | Write a new archive |
| Copying a member into another archive without unpacking it | `extract` and `add`, or for small members `add_data(member.name, source.open(member).read())` |
| A one-call "pack this directory" | Three lines: `create`, `defer close()`, `add`. They show what the names inside will be |
| A compression level on `tar.create` | `tar.Writer(gzip.open(path, write=true, level=9))` |
| Setting a member's modification time; an empty directory or a link from `add_data` | `add` takes times and kinds from disk |
| Owner and group, extended attributes, sparse files, device nodes | `exec.run(["tar", …])` for system backups; this module is for releases and data |
| `exclude=` or `skip=` on `add` and `extract` | A loop over `fs.walk(path, skip=…)` with `add`; a loop with `extract(member, dst)` |
| `strip=` on `extract` | The loop with `name=` in the tasks above, which also shows what happens to a member with no such component |
| Progress reporting | A loop with the method |
| Restoring ownership, set-id bits and directory times; links that point outside | Not offered: these are the unsafe parts |
