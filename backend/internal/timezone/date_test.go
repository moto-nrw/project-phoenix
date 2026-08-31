package timezone

import (
	"encoding/json"
	"testing"
	"time"
)

// TestDateFromTime_BoundaryInstants pins the exact failure window of the
// historical bug class: between 22:00/23:00 UTC and midnight UTC, the UTC
// calendar date is one day behind Berlin's.
func TestDateFromTime_BoundaryInstants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   time.Time
		want Date
	}{
		// Summer (CEST, UTC+2): Berlin flips to the next day at 22:00 UTC.
		{"summer 21:59 UTC same day", time.Date(2026, 6, 9, 21, 59, 0, 0, time.UTC), NewDate(2026, 6, 9)},
		{"summer 22:00 UTC next Berlin day", time.Date(2026, 6, 9, 22, 0, 0, 0, time.UTC), NewDate(2026, 6, 10)},
		{"summer 23:30 UTC next Berlin day", time.Date(2026, 6, 9, 23, 30, 0, 0, time.UTC), NewDate(2026, 6, 10)},
		// Winter (CET, UTC+1): the flip is at 23:00 UTC.
		{"winter 22:59 UTC same day", time.Date(2026, 1, 9, 22, 59, 0, 0, time.UTC), NewDate(2026, 1, 9)},
		{"winter 23:00 UTC next Berlin day", time.Date(2026, 1, 9, 23, 0, 0, 0, time.UTC), NewDate(2026, 1, 10)},
		// Berlin-local instants map to their own calendar day.
		{"berlin midnight", time.Date(2026, 6, 10, 0, 0, 0, 0, Berlin), NewDate(2026, 6, 10)},
		{"berlin 23:59", time.Date(2026, 6, 10, 23, 59, 59, 0, Berlin), NewDate(2026, 6, 10)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DateFromTime(tc.in); got != tc.want {
				t.Errorf("DateFromTime(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestDate_DSTArithmetic pins calendar arithmetic across the 2026 German
// DST transitions (Mar 29 spring-forward: 23h day; Oct 25 fall-back: 25h day).
func TestDate_DSTArithmetic(t *testing.T) {
	t.Parallel()

	if got := NewDate(2026, 3, 28).AddDays(2); got != NewDate(2026, 3, 30) {
		t.Errorf("AddDays across spring-forward = %v, want 2026-03-30", got)
	}
	if got := NewDate(2026, 10, 24).AddDays(2); got != NewDate(2026, 10, 26) {
		t.Errorf("AddDays across fall-back = %v, want 2026-10-26", got)
	}
	if got := NewDate(2026, 3, 28).DaysUntil(NewDate(2026, 3, 30)); got != 2 {
		t.Errorf("DaysUntil across spring-forward = %d, want 2", got)
	}
	if got := NewDate(2026, 10, 24).DaysUntil(NewDate(2026, 10, 26)); got != 2 {
		t.Errorf("DaysUntil across fall-back = %d, want 2", got)
	}
	if got := NewDate(2026, 4, 11).DaysUntil(NewDate(2026, 3, 28)); got != -14 {
		t.Errorf("negative DaysUntil = %d, want -14", got)
	}
	// Both transition days have a real 00:00 (DST jumps at 02:00 in Germany).
	for _, d := range []Date{NewDate(2026, 3, 29), NewDate(2026, 10, 25)} {
		bm := d.BerlinMidnight()
		if bm.Hour() != 0 || DateFromTime(bm) != d {
			t.Errorf("BerlinMidnight(%v) = %v, want 00:00 on the same day", d, bm)
		}
	}
	if got := NewDate(2026, 6, 10).Weekday(); got != time.Wednesday {
		t.Errorf("Weekday(2026-06-10) = %v, want Wednesday", got)
	}
	// AddDays normalizes month/year boundaries.
	if got := NewDate(2026, 12, 31).AddDays(1); got != NewDate(2027, 1, 1) {
		t.Errorf("AddDays year rollover = %v, want 2027-01-01", got)
	}
}

func TestDate_Comparisons(t *testing.T) {
	t.Parallel()

	a, b := NewDate(2026, 6, 9), NewDate(2026, 6, 10)
	if !a.Before(b) || b.Before(a) || !b.After(a) {
		t.Error("Before/After ordering wrong")
	}
	if a.Compare(b) != -1 || b.Compare(a) != 1 || a.Compare(a) != 0 {
		t.Error("Compare wrong")
	}
	if a != NewDate(2026, 6, 9) {
		t.Error("== equality wrong")
	}
	// Comparable: usable as a map key.
	set := map[Date]struct{}{a: {}, b: {}}
	if len(set) != 2 {
		t.Error("map keying wrong")
	}
}

// TestDate_StorageRepresentation pins the persistence-neutral representation
// consumed by the PostgreSQL adapter.
func TestDate_StorageRepresentation(t *testing.T) {
	t.Parallel()

	d := NewDate(2026, 6, 10)
	if string(d) != "2026-06-10" {
		t.Fatalf("storage value = %q; want 2026-06-10", d)
	}
	if !Date("").IsZero() {
		t.Fatal("empty storage value should be the zero Date")
	}
}

func TestDate_JSON(t *testing.T) {
	t.Parallel()

	d := NewDate(2026, 6, 10)

	b, err := json.Marshal(d)
	if err != nil || string(b) != `"2026-06-10"` {
		t.Fatalf("Marshal = %s, %v", b, err)
	}
	b, err = json.Marshal(Date(""))
	if err != nil || string(b) != "null" {
		t.Fatalf("Marshal zero = %s, %v", b, err)
	}
	// *Date nil → null via standard encoding.
	b, err = json.Marshal(struct {
		D *Date `json:"d"`
	}{})
	if err != nil || string(b) != `{"d":null}` {
		t.Fatalf("Marshal nil ptr = %s, %v", b, err)
	}

	var u Date
	if err := json.Unmarshal([]byte(`"2026-06-10"`), &u); err != nil || u != d {
		t.Fatalf("Unmarshal = %v, %v", u, err)
	}
	if err := json.Unmarshal([]byte("null"), &u); err != nil || !u.IsZero() {
		t.Fatalf("Unmarshal null = %v, %v; want zero", u, err)
	}
	// Strict format: reject sloppy and timestamp-shaped input.
	for _, bad := range []string{`"2026-6-1"`, `"2026-06-10T00:00:00Z"`, `"10.06.2026"`, `42`} {
		if err := json.Unmarshal([]byte(bad), &u); err == nil {
			t.Errorf("Unmarshal(%s) should error", bad)
		}
	}
}

func TestDate_StringAndFormat(t *testing.T) {
	t.Parallel()

	d := NewDate(2026, 6, 10)
	if d.String() != "2026-06-10" {
		t.Errorf("String() = %q", d.String())
	}
	if got := d.Format("02.01.2006"); got != "10.06.2026" {
		t.Errorf("Format German = %q", got)
	}
	if _, err := ParseDate("2026-06-10"); err != nil {
		t.Errorf("ParseDate valid: %v", err)
	}
	for _, bad := range []string{"2026-6-1", "2026-06-10T00:00:00Z", "", "garbage"} {
		if _, err := ParseDate(bad); err == nil {
			t.Errorf("ParseDate(%q) should error", bad)
		}
	}
	// NewDate normalizes out-of-range components like time.Date.
	if got := NewDate(2026, 1, 32); got != NewDate(2026, 2, 1) {
		t.Errorf("NewDate normalization = %v", got)
	}
	// Interop accessors.
	if got := d.EndOfDay(); got != time.Date(2026, 6, 10, 23, 59, 59, 0, Berlin) {
		t.Errorf("EndOfDay = %v", got)
	}
	if got := d.UTCMidnight(); got != time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC) {
		t.Errorf("UTCMidnight = %v", got)
	}
}
