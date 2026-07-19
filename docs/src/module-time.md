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
