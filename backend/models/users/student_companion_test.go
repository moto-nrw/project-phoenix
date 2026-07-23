package users

import (
	"errors"
	"testing"
)

// TestNewStudentCompanion_NormalizesOrder pins the storage invariant behind the
// DB CHECK (student_low_id < student_high_id): the pair is stored exactly once
// per weekday no matter which child the caller edits, so removing the link from
// one card removes the same row from the other.
func TestNewStudentCompanion_NormalizesOrder(t *testing.T) {
	const (
		lower  = int64(41)
		higher = int64(77)
	)

	tests := []struct {
		name              string
		studentID         int64
		companionID       int64
		expectedLowValue  int64
		expectedHighValue int64
	}{
		{
			name:              "already in low/high order",
			studentID:         lower,
			companionID:       higher,
			expectedLowValue:  lower,
			expectedHighValue: higher,
		},
		{
			name:              "reversed arguments are swapped",
			studentID:         higher,
			companionID:       lower,
			expectedLowValue:  lower,
			expectedHighValue: higher,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edge, err := NewStudentCompanion(tt.studentID, tt.companionID, 3)
			if err != nil {
				t.Fatalf("NewStudentCompanion() unexpected error: %v", err)
			}
			if edge.StudentLowID != tt.expectedLowValue {
				t.Errorf("StudentLowID = %d, want %d", edge.StudentLowID, tt.expectedLowValue)
			}
			if edge.StudentHighID != tt.expectedHighValue {
				t.Errorf("StudentHighID = %d, want %d", edge.StudentHighID, tt.expectedHighValue)
			}
			if edge.StudentLowID >= edge.StudentHighID {
				t.Errorf("low must be strictly smaller than high, got %d/%d", edge.StudentLowID, edge.StudentHighID)
			}
			if edge.Weekday != 3 {
				t.Errorf("Weekday = %d, want 3", edge.Weekday)
			}
		})
	}
}

// TestNewStudentCompanion_Rejects covers the constructor's guard rails: the
// invalid combinations must never reach the database, where they would either
// violate the CHECK constraint or silently store an unreadable weekday.
func TestNewStudentCompanion_Rejects(t *testing.T) {
	tests := []struct {
		name        string
		studentID   int64
		companionID int64
		weekday     int
		wantErr     error
	}{
		{
			name:        "self link",
			studentID:   12,
			companionID: 12,
			weekday:     1,
			wantErr:     ErrCompanionSelfLink,
		},
		{
			name:        "weekday below range",
			studentID:   12,
			companionID: 13,
			weekday:     0,
			wantErr:     ErrCompanionInvalidWeekday,
		},
		{
			name:        "weekend weekday",
			studentID:   12,
			companionID: 13,
			weekday:     6,
			wantErr:     ErrCompanionInvalidWeekday,
		},
		{
			name:        "weekday above range",
			studentID:   12,
			companionID: 13,
			weekday:     9,
			wantErr:     ErrCompanionInvalidWeekday,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edge, err := NewStudentCompanion(tt.studentID, tt.companionID, tt.weekday)
			if err == nil {
				t.Fatalf("NewStudentCompanion() expected an error, got edge %+v", edge)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want errors.Is(..., %v)", err, tt.wantErr)
			}
			if edge != nil {
				t.Errorf("expected a nil edge on error, got %+v", edge)
			}
		})
	}
}

// TestNewStudentCompanion_RejectsMissingIDs guards the zero/negative id case,
// which would otherwise insert a row pointing at no child at all.
func TestNewStudentCompanion_RejectsMissingIDs(t *testing.T) {
	for _, tt := range []struct {
		name        string
		studentID   int64
		companionID int64
	}{
		{name: "missing student", studentID: 0, companionID: 5},
		{name: "missing companion", studentID: 5, companionID: 0},
		{name: "negative id", studentID: -1, companionID: 5},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewStudentCompanion(tt.studentID, tt.companionID, 1); err == nil {
				t.Fatal("expected an error for a missing student ID")
			}
		})
	}
}

// TestStudentCompanion_Other checks the far-end lookup from both endpoints and
// from a child that is not part of the edge at all — the "ok" flag is what
// keeps unrelated edges out of a child's companion list.
func TestStudentCompanion_Other(t *testing.T) {
	edge, err := NewStudentCompanion(10, 20, 1)
	if err != nil {
		t.Fatalf("NewStudentCompanion() unexpected error: %v", err)
	}

	t.Run("from the low endpoint", func(t *testing.T) {
		other, ok := edge.Other(10)
		if !ok {
			t.Fatal("expected the low endpoint to be part of the edge")
		}
		if other != 20 {
			t.Errorf("Other(10) = %d, want 20", other)
		}
	})

	t.Run("from the high endpoint", func(t *testing.T) {
		other, ok := edge.Other(20)
		if !ok {
			t.Fatal("expected the high endpoint to be part of the edge")
		}
		if other != 10 {
			t.Errorf("Other(20) = %d, want 10", other)
		}
	})

	t.Run("from a non-member", func(t *testing.T) {
		other, ok := edge.Other(30)
		if ok {
			t.Error("a child outside the edge must report ok=false")
		}
		if other != 0 {
			t.Errorf("Other(30) = %d, want 0", other)
		}
	})
}

// TestCompanionWeekdayNumber pins the key<->number translation both API
// directions depend on.
func TestCompanionWeekdayNumber(t *testing.T) {
	for key, want := range map[string]int{
		PickupDayMonday:    1,
		PickupDayTuesday:   2,
		PickupDayWednesday: 3,
		PickupDayThursday:  4,
		PickupDayFriday:    5,
	} {
		got, err := CompanionWeekdayNumber(key)
		if err != nil {
			t.Fatalf("CompanionWeekdayNumber(%q) unexpected error: %v", key, err)
		}
		if got != want {
			t.Errorf("CompanionWeekdayNumber(%q) = %d, want %d", key, got, want)
		}
		if back := CompanionWeekdayKeys[want]; back != key {
			t.Errorf("CompanionWeekdayKeys[%d] = %q, want %q", want, back, key)
		}
	}

	if _, err := CompanionWeekdayNumber("sat"); !errors.Is(err, ErrCompanionInvalidWeekday) {
		t.Errorf("CompanionWeekdayNumber(\"sat\") error = %v, want ErrCompanionInvalidWeekday", err)
	}
}

// TestCompanionLinksFromEdges_FoldsWeekdays is the core read-model contract:
// several edges for the same companion collapse into ONE entry whose weekdays
// are ordered Mon..Fri (not insertion order, not numeric-string order), and
// edges that do not touch the child are dropped.
func TestCompanionLinksFromEdges_FoldsWeekdays(t *testing.T) {
	const (
		student   = int64(100)
		companion = int64(50)
		other     = int64(200)
		stranger1 = int64(700)
		stranger2 = int64(800)
	)

	// Deliberately out of order (fri, mon, wed) to prove the fold sorts.
	edges := []*StudentCompanion{
		mustEdge(t, student, companion, 5),
		mustEdge(t, student, companion, 1),
		mustEdge(t, companion, student, 3), // reversed arguments, same pair
		mustEdge(t, student, other, 2),
		mustEdge(t, stranger1, stranger2, 1), // unrelated edge, must be ignored
		nil,                                  // nil entries must be skipped, not panic
	}

	links := CompanionLinksFromEdges(student, edges)

	if len(links) != 2 {
		t.Fatalf("expected 2 companions, got %d (%+v)", len(links), links)
	}

	// Links are sorted by companion id: 50 before 200.
	if links[0].CompanionStudentID != companion || links[1].CompanionStudentID != other {
		t.Fatalf("links not sorted by companion id: %+v", links)
	}

	wantWeekdays := []string{PickupDayMonday, PickupDayWednesday, PickupDayFriday}
	if len(links[0].Weekdays) != len(wantWeekdays) {
		t.Fatalf("weekdays = %v, want %v", links[0].Weekdays, wantWeekdays)
	}
	for i, want := range wantWeekdays {
		if links[0].Weekdays[i] != want {
			t.Errorf("weekday[%d] = %q, want %q (full: %v)", i, links[0].Weekdays[i], want, links[0].Weekdays)
		}
	}

	if len(links[1].Weekdays) != 1 || links[1].Weekdays[0] != PickupDayTuesday {
		t.Errorf("second companion weekdays = %v, want [%s]", links[1].Weekdays, PickupDayTuesday)
	}
}

// TestCompanionLinksFromEdges_NoMatches pins the empty (non-nil) result the API
// layer serializes as [] rather than null.
func TestCompanionLinksFromEdges_NoMatches(t *testing.T) {
	edges := []*StudentCompanion{mustEdge(t, 700, 800, 1)}

	links := CompanionLinksFromEdges(100, edges)
	if links == nil {
		t.Fatal("expected a non-nil empty slice")
	}
	if len(links) != 0 {
		t.Errorf("expected no links, got %+v", links)
	}
}

func mustEdge(t *testing.T, studentID, companionID int64, weekday int) *StudentCompanion {
	t.Helper()
	edge, err := NewStudentCompanion(studentID, companionID, weekday)
	if err != nil {
		t.Fatalf("NewStudentCompanion(%d, %d, %d) failed: %v", studentID, companionID, weekday, err)
	}
	return edge
}

// TestCompanionLinksFingerprint_OrderIndependent pins that the fingerprint
// describes the SET of links: a re-fetch handing the same links back in another
// order (or with the weekdays shuffled) must not read as a foreign change and
// refuse a legitimate save.
func TestCompanionLinksFingerprint_OrderIndependent(t *testing.T) {
	a := []CompanionLink{
		{CompanionStudentID: 42, Weekdays: []string{PickupDayMonday, PickupDayWednesday}},
		{CompanionStudentID: 7, Weekdays: []string{PickupDayFriday}},
	}
	b := []CompanionLink{
		{CompanionStudentID: 7, Weekdays: []string{PickupDayFriday}},
		{CompanionStudentID: 42, Weekdays: []string{PickupDayWednesday, PickupDayMonday}},
	}

	if got, want := CompanionLinksFingerprint(a), CompanionLinksFingerprint(b); got != want {
		t.Errorf("fingerprint = %q, want %q", got, want)
	}
}

// TestCompanionLinksFingerprint_MirrorsFrontendFormat pins the exact string
// shape, because the client echoes a fingerprint produced by
// companionsFingerprint() in frontend/src/lib/student-companion-api.ts. A
// silent format change on either side would make every save look stale.
func TestCompanionLinksFingerprint_MirrorsFrontendFormat(t *testing.T) {
	links := []CompanionLink{
		{CompanionStudentID: 42, Weekdays: []string{PickupDayWednesday, PickupDayMonday}},
		{CompanionStudentID: 7, Weekdays: []string{PickupDayFriday}},
	}

	if got, want := CompanionLinksFingerprint(links), "42:mon,wed|7:fri"; got != want {
		t.Errorf("fingerprint = %q, want %q", got, want)
	}
	if got := CompanionLinksFingerprint(nil); got != "" {
		t.Errorf("empty fingerprint = %q, want \"\"", got)
	}
}

// TestCompanionLinksFingerprint_DetectsChange pins the case the check exists
// for: a link someone else added must not fingerprint like the list without it.
func TestCompanionLinksFingerprint_DetectsChange(t *testing.T) {
	loaded := []CompanionLink{{CompanionStudentID: 7, Weekdays: []string{PickupDayFriday}}}
	stored := []CompanionLink{
		{CompanionStudentID: 7, Weekdays: []string{PickupDayFriday}},
		{CompanionStudentID: 9, Weekdays: []string{PickupDayFriday}},
	}

	if CompanionLinksFingerprint(loaded) == CompanionLinksFingerprint(stored) {
		t.Error("an added link must change the fingerprint")
	}
	// A weekday added to an EXISTING link counts as a change too.
	widened := []CompanionLink{{CompanionStudentID: 7, Weekdays: []string{PickupDayThursday, PickupDayFriday}}}
	if CompanionLinksFingerprint(loaded) == CompanionLinksFingerprint(widened) {
		t.Error("an added weekday must change the fingerprint")
	}
}

// TestFormatCompanionLinks pins the offline-list rendering: names carry the
// weekdays unless the link covers the whole week, so a Monday-only
// Laufgemeinschaft cannot be read off a paper list as a standing arrangement.
func TestFormatCompanionLinks(t *testing.T) {
	got := FormatCompanionLinks([]CompanionLink{
		{CompanionStudentID: 7, FirstName: "Mia", LastName: "Schulz", Weekdays: []string{PickupDayMonday, PickupDayTuesday}},
		{CompanionStudentID: 9, FirstName: "Tom", LastName: "Meier", Weekdays: PickupDayOrder},
	})
	if want := "Mia Schulz (Mo, Di), Tom Meier"; got != want {
		t.Errorf("FormatCompanionLinks = %q, want %q", got, want)
	}

	if got := FormatCompanionLinks(nil); got != "" {
		t.Errorf("empty render = %q, want \"\"", got)
	}

	// A link whose names were not joined in still names the child by id rather
	// than rendering an empty "(Mo)".
	if got, want := FormatCompanionLinks([]CompanionLink{
		{CompanionStudentID: 12, Weekdays: []string{PickupDayMonday}},
	}), "Kind #12 (Mo)"; got != want {
		t.Errorf("nameless render = %q, want %q", got, want)
	}
}
