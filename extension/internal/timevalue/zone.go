package timevalue

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	stdtime "time"

	"github.com/aisk/goblin/object"
)

// loaded caches the IANA zones LoadLocation has read, so a zone named in a
// loop is read from the database once.
var loaded sync.Map // string -> *time.Location

// Zone resolves a zone spelling to a Location, the one resolver behind
// Time(zone=), parse(zone=) and to_zone: "local", "UTC", an IANA name, or a
// fixed offset "±HH:MM". fn names the function in the ValueError for
// anything else.
func Zone(fn, name string) (*stdtime.Location, error) {
	switch name {
	case "local":
		return stdtime.Local, nil
	case "UTC":
		return stdtime.UTC, nil
	case "", "Local":
		// Go treats these specially; each zone keeps one spelling.
		return nil, unknownZone(fn, name)
	}
	if offset, ok := fixedOffset(name); ok {
		return stdtime.FixedZone(name, offset), nil
	}
	if loc, ok := loaded.Load(name); ok {
		return loc.(*stdtime.Location), nil
	}
	loc, err := stdtime.LoadLocation(name)
	if err != nil || !spelledAsOnDisk(name) {
		return nil, unknownZone(fn, name)
	}
	loaded.Store(name, loc)
	return loc, nil
}

// zoneDirs are the directories Go's LoadLocation reads the system database
// from, after $ZONEINFO.
var zoneDirs = []string{"/usr/share/zoneinfo/", "/usr/share/lib/zoneinfo/", "/usr/lib/locale/TZ/", "/etc/zoneinfo/"}

// spelledAsOnDisk reports whether name, which LoadLocation accepted, is
// spelled as the zone's file is. On a case-insensitive file system (macOS)
// LoadLocation also finds "asia/shanghai", which Linux and Go's embedded
// database refuse, and the Time would report that spelling as its zone. A
// zone that is not in a system directory came from Go's own database,
// whose lookup is exact.
func spelledAsOnDisk(name string) bool {
	dirs := zoneDirs
	if env := os.Getenv("ZONEINFO"); env != "" {
		dirs = append([]string{env}, dirs...)
	}
	for _, dir := range dirs {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			continue
		}
		for _, part := range strings.Split(name, "/") {
			entries, err := os.ReadDir(dir)
			if err != nil || !slices.ContainsFunc(entries, func(e os.DirEntry) bool { return e.Name() == part }) {
				return false
			}
			dir = filepath.Join(dir, part)
		}
		return true
	}
	return true
}

func unknownZone(fn, name string) error {
	return object.NewValueError("%s() unknown time zone %s", fn, strconv.Quote(name))
}

// fixedOffset reads "±HH:MM" as seconds east of UTC.
func fixedOffset(name string) (int, bool) {
	if len(name) != 6 || (name[0] != '+' && name[0] != '-') || name[3] != ':' {
		return 0, false
	}
	hours, ok1 := twoDigits(name[1:3])
	minutes, ok2 := twoDigits(name[4:6])
	if !ok1 || !ok2 || hours > 23 || minutes > 59 {
		return 0, false
	}
	offset := hours*3600 + minutes*60
	if name[0] == '-' {
		offset = -offset
	}
	return offset, true
}

func twoDigits(s string) (int, bool) {
	if len(s) != 2 || s[0] < '0' || s[0] > '9' || s[1] < '0' || s[1] > '9' {
		return 0, false
	}
	return int(s[0]-'0')*10 + int(s[1]-'0'), true
}

// offsetZone is the zone of a parsed offset: UTC for zero written as "Z",
// otherwise the fixed zone named by its "±HH:MM" spelling, so Time.zone
// reports what a zone= keyword accepts.
func offsetZone(seconds int) *stdtime.Location {
	return stdtime.FixedZone(offsetText(seconds, true), seconds)
}

// offsetText spells seconds east of UTC as ±HHMM, or ±HH:MM with colon.
func offsetText(seconds int, colon bool) string {
	sign := byte('+')
	if seconds < 0 {
		sign = '-'
		seconds = -seconds
	}
	minutes := seconds / 60
	b := []byte{sign}
	b = appendPadded(b, minutes/60, 2, '0')
	if colon {
		b = append(b, ':')
	}
	b = appendPadded(b, minutes%60, 2, '0')
	return string(b)
}

// ZoneName is the spelling Time.zone reports for loc.
func ZoneName(loc *stdtime.Location) string {
	switch loc {
	case stdtime.Local:
		return "local"
	case stdtime.UTC:
		return "UTC"
	}
	return loc.String()
}

// ZoneOr resolves an optional `zone=` argument read with NullableStr:
// absent or nil is the local zone.
func ZoneOr(fn string, zone object.String, given bool) (*stdtime.Location, error) {
	if !given {
		return stdtime.Local, nil
	}
	return Zone(fn, string(zone))
}
