# time

The time module creates, parses, formats, and measures time values.

~~~goblin
import "time"

var started = time.now()
time.sleep(0.1)
print(time.since(started))
print(started.year)
print(started.format("2006-01-02"))
~~~

now() returns the current local time. sleep(seconds) pauses for an integer or
float number of seconds. since(value) returns elapsed seconds as a Float.

Use parse(layout, text) to parse formatted text. Layouts use Go reference time
formatting, so 2006-01-02 represents a year-month-day format. unix(seconds)
creates a time from a Unix timestamp.

~~~goblin
var day = time.parse("2006-01-02", "2026-07-19")
print(day.weekday)
print(time.unix(day.unix))
~~~

Invalid parsing raises ParseError.

## Time fields and formatting

Time values provide year, month, day, hour, minute, second, nanosecond, unix,
unix_nano, and weekday fields. format(layout) turns a Time into text using the
same Go reference-layout convention as parse().

~~~goblin
var now = time.now()
print(now.year, now.month, now.day)
print(now.weekday)
print(now.format("2006-01-02 15:04:05"))
~~~

Time values can be compared with the ordinary comparison operators. Use this
for expiration and scheduling checks; use since() when the needed result is a
duration in seconds.

~~~goblin
var deadline = time.unix(2000000000)
if time.now() > deadline {
    print("expired")
}
~~~

sleep() blocks the current execution path. It is appropriate for a deliberate
delay or simple retry loop, but it is not a substitute for channel-based
coordination between spawned functions.
