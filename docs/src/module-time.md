# time

The time module reads the clock, sleeps, and works with instants and time
zones. There are two kinds of value and no more: an **instant** is a `Time`,
and a **duration** is a number of seconds (an Int or a Float), as it is
everywhere else in Goblin (`timeout=`, `Chan.receive`). `time.MINUTE`,
`time.HOUR` and `time.DAY` are the readable multipliers.

## Common tasks

~~~goblin
import "fs"
import "math"
import "time"

# Timestamps
print(time.now())                                   # 2026-07-19T09:30:00+08:00
print(time.now().format("%Y-%m-%d %H:%M"))          # 2026-07-19 09:30
var stamp = time.now().format("%Y%m%d")
var name = "backup-{stamp}.tar"
print("[{time.now().format(`%H:%M:%S`)}] started")  # inside an interpolation the layout is a raw string
print(time.now().to_utc())                          # 2026-07-19T01:30:00Z

# How long did it take
var started = time.now()
build()
print("built in {time.now() - started:.1f} seconds")

# Dates from data
var due = time.parse("2026-08-01")
var posted = time.parse("19/07/2026 14:05", layout="%d/%m/%Y %H:%M")
if time.now() > due {
    print("overdue by {(time.now() - due) / time.DAY:.0f} days")
}

# A column of dates, some of them wrong
func read_date(text) {
    try {
        return time.parse(text, layout="%d/%m/%Y")
    } catch e: ParseError {
        eprint("not a date: {e.text}")
        return nil
    }
}

# Relative times
var last_week = time.now() - 7 * time.DAY
var stale = fs.stat("cache.json").modified_at < last_week

# What time is that for someone else
var meeting = time.Time(2026, 7, 19, hour=9, minute=30, zone="America/New_York")
print(meeting.to_zone("Asia/Shanghai"))                    # 2026-07-19T21:30:00+08:00
print(meeting.to_zone("Europe/Paris").format("%H:%M %Z"))  # 15:30 CEST
print(meeting.zone, meeting.offset / time.HOUR)            # America/New_York -4
var record_at = meeting.to_utc().format("%Y-%m-%dT%H:%M:%SZ")   # 2026-07-19T13:30:00Z, for a log or an API

# A time an API gave in UTC, shown to the person at this computer
print(time.parse("2026-07-19T13:30:00Z").to_zone("local").format("%H:%M"))

# Times in data that do not say their zone
var opened = time.parse("2026-07-19 09:00", zone="Europe/Paris")
var logged = time.parse(record["created"], zone="UTC")
var changed = time.parse(last_modified, layout="%a, %d %b %Y %H:%M:%S GMT", zone="UTC")

# A zone from settings, which may be misspelt
func show(t, zone) {
    try {
        return t.to_zone(zone).format("%Y-%m-%d %H:%M")
    } catch e: ValueError {
        eprint(e.message)                  # to_zone() unknown time zone "Asia/Shangai"
        return t.format("%Y-%m-%d %H:%M")
    }
}

# Today, and what happened on which day
var today = time.now().start_of_day()
var fresh = fs.stat("report.html").modified_at >= today
var per_day = {}
for event in events {
    var day = time.unix(event["at"]).start_of_day()
    per_day[day] = per_day.get(day, default=0) + 1
}

# Whole days between two dates, whatever the hours
var nights = math.round((checkout.start_of_day() - checkin.start_of_day()) / time.DAY)

# Calendar values and calendar steps: the constructor rolls over
var launch = time.Time(2026, 7, 19, hour=9, minute=30)
if launch.weekday >= 6 {
    print("that is a weekend")
}
var next_month = time.Time(launch.year, launch.month + 1, 1)
var month_end = time.Time(launch.year, launch.month + 1, 0)
var same_time_next_week = time.Time(launch.year, launch.month, launch.day + 7, hour=launch.hour, minute=launch.minute, zone=launch.zone)

# Which week is it
print(time.now().format("%G-W%V"))                  # 2026-W29

# Unix timestamps from and for an API
var created = time.unix(payload["created"])
var expires = (time.now() + time.HOUR).unix

# Retry with a pause
func fetch_with_retry() {
    for attempt in range(3) {
        try {
            return fetch()
        } catch e: IOError {
            time.sleep(2)
        }
    }
    raise Error("gave up after 3 attempts")
}
~~~

`docs/examples/time-tasks.goblin` runs these tasks against fixed instants;
`examples/time.goblin` covers the rest of the module.

## Module members

| Member | Returns | Behaviour |
| --- | --- | --- |
| `now()` | Time | The current instant, in the local zone |
| `sleep(seconds)` | `nil` | Pauses this goblin for an Int or Float number of seconds; `0` returns at once. ValueError for a negative number |
| `Time(year, month, day, hour=0, minute=0, second=0, zone=nil)` | Time | That wall-clock time in `zone`; `nil` is the local zone. Out-of-range parts roll over: `month=13` is January of the next year, `day=0` the last day of the previous month. TypeError for a part that is not an Int |
| `parse(text, layout=nil, zone=nil)` | Time | Without a layout, ISO 8601 / RFC 3339 (below). With one, the layout's form exactly. Text that carries an offset keeps it; text that carries none is wall-clock time in `zone`, `nil` being the local zone |
| `unix(seconds)` | Time | The instant that many seconds (Int or Float) after 1970-01-01 UTC, in the local zone. ValueError outside the years 1 to 9999 |
| `MINUTE`, `HOUR`, `DAY` | Int | `60`, `3600`, `86400`: `time.sleep(5 * time.MINUTE)` |

The forms `parse(text)` accepts with no layout are `2026-07-19`,
`2026-07-19T09:30`, `2026-07-19T09:30:15` and `2026-07-19T09:30:15.250`, each
time form optionally followed by `Z` or an offset such as `+08:00`; a space is
accepted in place of `T`.

## Zones

A zone is a String, written the same four ways wherever one is accepted
(`Time(zone=)`, `parse(zone=)`, `to_zone`) and wherever one is reported
(`Time.zone`):

| Spelling | Zone |
| --- | --- |
| `"local"` | The computer's zone (the `TZ` environment variable, else the system setting) |
| `"UTC"` | UTC |
| `"Asia/Shanghai"`, `"America/New_York"`, … | A zone of the IANA time zone database, by its name exactly as listed there. Its offset follows that place's rules, daylight saving included |
| `"+08:00"`, `"-03:30"` | A fixed offset from UTC, always `±HH:MM` |

- A `zone=` keyword also accepts `nil`, meaning `"local"`.
- Anything else is `ValueError: to_zone() unknown time zone "Asia/Shangai"`,
  with the function's own name in front. It carries no data: the caller holds
  the name.
- The zone database is built into every program that imports `time`, so a
  name works the same on Windows, in a minimal container and on a machine
  whose system database is old or missing. The system's database is used when
  it is there and the built-in copy is the fallback. It costs about 450 KB in
  a compiled executable that imports `time`; programs that do not import it
  pay nothing.
- A wall-clock time that does not exist (the hour skipped when clocks go
  forward) or exists twice (the hour repeated when they go back) gives one of
  the two possible instants. Scripts that schedule inside those hours should
  use UTC.

## Time

Immutable: an instant together with the zone it is shown in.

`==` and the ordering operators compare **instants**, whatever the zones
(09:00+08:00 equals 01:00Z), and a Time can be a Dict key on the same terms.
A Time prints as RFC 3339 (`2026-07-19T09:30:00+08:00`), with fractional
seconds only when they are not zero; its zone's name is not shown, its offset
is. Like every library value it is always truthy.

| Member | Kind | Value |
| --- | --- | --- |
| `year`, `month`, `day` | property | Int; `month` is 1–12 |
| `hour`, `minute`, `second` | property | Int |
| `nanosecond` | property | Int 0–999999999, the fraction of the second |
| `weekday` | property | Int, 1 (Monday) to 7 (Sunday), as ISO 8601. The name is `t.format("%A")` |
| `unix` | property | Int, whole seconds since 1970-01-01 UTC, rounded down |
| `zone` | property | String: the zone this Time is shown in, in a spelling of the table above (`"local"`, `"UTC"`, `"Europe/Paris"`, `"+08:00"`). Passing it to `zone=` gives the same zone |
| `offset` | property | Int: seconds east of UTC at this instant in this zone (`28800` for `+08:00`, negative west) |
| `format(layout=nil)` | method | String. `nil` gives the printed form; otherwise per the layout. ValueError for an unknown directive |
| `to_utc()` | method | The same instant shown in UTC: exactly `t.to_zone("UTC")` |
| `to_zone(zone)` | method | The same instant shown in `zone`. `t.to_zone("local")` shows it in the computer's zone. ValueError for an unknown zone |
| `start_of_day()` | method | 00:00:00 of the same calendar day, in the same zone |

All the wall-clock properties are read in the Time's zone. `Time.zone` reports
`"local"` for the local zone rather than guessing its IANA name, which the
operating system does not reliably tell a program.

| Expression | Result |
| --- | --- |
| `t + seconds`, `seconds + t`, `t - seconds` | A Time shifted by exactly that many elapsed seconds, in the same zone. ValueError outside the representable range |
| `t - u` | Float, the seconds from `u` to `t`. Between two `now()` values of one process it is measured on the monotonic clock |
| `t < u`, `t == u`, … | By instant |

`t + time.DAY` is 24 hours later, which is not always "the same time
tomorrow" in a zone with daylight saving. A calendar step is the constructor,
which rolls over: `time.Time(t.year, t.month, t.day + 1, hour=t.hour,
minute=t.minute, zone=t.zone)`.

## Layouts

Layouts are `strftime` directives, the form shared by C, Python, Ruby and
`date(1)`. Anything else in a layout is literal text, so `"2006 Jan"` is just
those characters.

| Directive | Meaning | Example |
| --- | --- | --- |
| `%Y` / `%y` | year, 4 / 2 digits | `2026` / `26` |
| `%m` | month 01–12 | `07` |
| `%d` / `%e` | day 01–31 / space padded | `09` / ` 9` |
| `%H` / `%I` | hour 00–23 / 01–12 | `15` / `03` |
| `%M`, `%S` | minute, second | `04`, `05` |
| `%f` | microseconds, 6 digits | `250000` |
| `%p` | `AM` / `PM` | `PM` |
| `%A` / `%a` | weekday name, full / 3 letters (English) | `Sunday` / `Sun` |
| `%B` / `%b` | month name, full / 3 letters (English) | `July` / `Jul` |
| `%j` | day of the year 001–366 | `200` |
| `%z` | offset | `+0800` |
| `%Z` | the zone's abbreviation at that instant; the offset where the zone has none. `format` only | `CST`, `EDT`, `UTC` |
| `%G`, `%V` | ISO 8601 week-numbering year and week 01–53. `format` only | `2026`, `29` |
| `%%` | a literal `%` | `%` |

`parse` accepts the same directives, except the three marked `format` only,
which are a ValueError in a `parse` layout: an abbreviation does not name a
zone (`CST` is three of them) and a week does not name a day. When parsing:

- names (`%A`, `%a`, `%B`, `%b`, `%p`) are read case-insensitively; `%j`,
  `%a` and `%A` are read and then ignored;
- two-digit fields accept one digit as well (`9/7/2026` with `%d/%m/%Y`);
  `%Y` needs four digits; `%y` reads 69–99 as 1969–1999 and 00–68 as
  2000–2068;
- `%z` reads `+0800`, `+08:00` or `Z`; a layout without it reads the text in
  `zone`;
- a part the layout does not mention is taken from 1900-01-01 00:00:00, as in
  C and Python: `time.parse("09:30", layout="%H:%M")` is on 1900-01-01;
- text naming a day that does not exist (`31/06/2026`) does not match: unlike
  the constructor, parsing does not roll over.

A layout is not a format specifier: `"{t:%Y}"` is rejected when the program is
parsed, because every value other than a number takes only fill, alignment and
width after `:`. Call `format` and pad its result instead:

~~~goblin
print("{t:%H:%M}")                        # not valid
print("{t.format(`%H:%M`):>8}")           # valid: a method call, then padding
~~~

## Errors

| Failure | Raises | Data |
| --- | --- | --- |
| `parse(text, layout=…)` of text that does not fit | ParseError `parse() "19/07" does not match layout "%d/%m/%Y"` | `text`, `layout` |
| `parse(text)` of text that is not ISO 8601 | ParseError `parse() "19/07" is not an ISO 8601 time` | `text`, and `layout` as `nil` |
| An unknown directive | ValueError `format() unknown layout directive %Q` | — |
| `%Z`, `%G` or `%V` in a `parse` layout | ValueError `parse() layout directive %Z can only be used to format` | — |
| An unknown zone | ValueError `to_zone() unknown time zone "Asia/Shangai"` | — |
| A negative `sleep`, a `unix` out of range, arithmetic out of range | ValueError | — |

A bad layout or zone is a mistake in the program rather than in the data, so
it is a ValueError and is reported whatever the text.

## Left out

A Duration type (a duration is a number of seconds, and a format specifier
shows it: `"{elapsed:.1f} s"`); calendar arithmetic as a method ("add one
month": rebuild with `Time(...)`, which rolls over); `to_local()` (it is
`to_zone("local")`); listing the known zone names; parsing zone
abbreviations; choosing between the two readings of an ambiguous wall-clock
time; localised month and weekday names; a layout as a format specifier; a
`to_json()` on Time (write `t.format()` or `t.unix` where you encode).
