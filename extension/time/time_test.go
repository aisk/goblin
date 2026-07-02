package time

import (
	"testing"
	stdtime "time"

	"github.com/aisk/goblin/object"
)

func TestNow(t *testing.T) {
	result, err := now(object.CallArgs{})
	if err != nil {
		t.Fatalf("now() returned error: %v", err)
	}
	tm, ok := result.(*Time)
	if !ok {
		t.Fatalf("now() did not return *Time, got %T", result)
	}
	if !tm.Bool() {
		t.Fatal("now() returned zero time")
	}
}

func TestParseAndFormat(t *testing.T) {
	result, err := parse(object.CallArgs{
		Positional: []object.Object{
			object.String("2006-01-02"),
			object.String("2026-07-02"),
		},
	})
	if err != nil {
		t.Fatalf("parse() returned error: %v", err)
	}
	tm, ok := result.(*Time)
	if !ok {
		t.Fatalf("parse() did not return *Time, got %T", result)
	}

	// Check attributes
	year, _ := tm.GetAttr("year")
	if year.(object.Integer) != 2026 {
		t.Errorf("expected year 2026, got %v", year)
	}
	month, _ := tm.GetAttr("month")
	if month.(object.Integer) != 7 {
		t.Errorf("expected month 7, got %v", month)
	}

	// Check format method
	formatFn, err := tm.GetAttr("format")
	if err != nil {
		t.Fatalf("GetAttr(format) error: %v", err)
	}
	fn, ok := formatFn.(*object.Function)
	if !ok {
		t.Fatalf("format is not *Function")
	}
	formatted, err := fn.Fn(object.CallArgs{
		Positional: []object.Object{object.String("2006/01/02")},
	})
	if err != nil {
		t.Fatalf("format() error: %v", err)
	}
	if string(formatted.(object.String)) != "2026/07/02" {
		t.Errorf("expected 2026/07/02, got %v", formatted)
	}
}

func TestUnix(t *testing.T) {
	result, err := unix(object.CallArgs{
		Positional: []object.Object{object.Integer(0)},
	})
	if err != nil {
		t.Fatalf("unix() returned error: %v", err)
	}
	tm, ok := result.(*Time)
	if !ok {
		t.Fatalf("unix() did not return *Time, got %T", result)
	}
	unixVal, _ := tm.GetAttr("unix")
	if unixVal.(object.Integer) != 0 {
		t.Errorf("expected unix 0, got %v", unixVal)
	}
}

func TestSince(t *testing.T) {
	start := NewTime(parseTimeHelper(t))
	elapsed, err := since(object.CallArgs{
		Positional: []object.Object{start},
	})
	if err != nil {
		t.Fatalf("since() returned error: %v", err)
	}
	f, ok := elapsed.(object.Float)
	if !ok {
		t.Fatalf("since() did not return Float, got %T", elapsed)
	}
	if f <= 0 {
		t.Errorf("expected positive elapsed time, got %v", f)
	}
}

func TestTimeCompare(t *testing.T) {
	t1 := NewTime(parseStrHelper(t, "2025-01-01"))
	t2 := NewTime(parseStrHelper(t, "2026-01-01"))

	cmp, err := t1.Compare(t2)
	if err != nil {
		t.Fatalf("Compare error: %v", err)
	}
	if cmp != -1 {
		t.Errorf("expected t1 < t2 = -1, got %d", cmp)
	}

	cmp2, err := t2.Compare(t1)
	if err != nil {
		t.Fatalf("Compare error: %v", err)
	}
	if cmp2 != 1 {
		t.Errorf("expected t2 > t1 = 1, got %d", cmp2)
	}
}

func parseTimeHelper(t *testing.T) (tm stdtime.Time) {
	tm, err := stdtime.Parse("2006-01-02", "2025-01-01")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	return
}

func parseStrHelper(t *testing.T, s string) stdtime.Time {
	tm, err := stdtime.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	return tm
}
