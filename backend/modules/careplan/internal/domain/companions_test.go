package domain

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// TestNewCompanionEdge_NormalizesOrder pins the storage invariant behind the
// DB CHECK (student_low_id < student_high_id): the pair is stored exactly once
// per weekday no matter which child the caller edits, so removing the link from
// one card removes the same row from the other.
func TestNewCompanionEdge_NormalizesOrder(t *testing.T) {
	t.Parallel()

	const (
		lower  = int64(41)
		higher = int64(77)
	)
	for _, tt := range []struct {
		name                   string
		studentID, companionID int64
	}{
		{name: "already in low/high order", studentID: lower, companionID: higher},
		{name: "reversed arguments are swapped", studentID: higher, companionID: lower},
	} {
		t.Run(tt.name, func(t *testing.T) {
			edge, err := NewCompanionEdge(tt.studentID, tt.companionID, 3)
			if err != nil {
				t.Fatalf("NewCompanionEdge() unexpected error: %v", err)
			}
			if edge.StudentLowID != lower || edge.StudentHighID != higher {
				t.Errorf("edge = %d/%d, want %d/%d", edge.StudentLowID, edge.StudentHighID, lower, higher)
			}
			if edge.Weekday != 3 {
				t.Errorf("Weekday = %d, want 3", edge.Weekday)
			}
		})
	}
}

// TestNewCompanionEdge_Rejects covers the constructor's guard rails: the
// invalid combinations must never reach the database, where they would either
// violate the CHECK constraint or silently store an unreadable weekday.
func TestNewCompanionEdge_Rejects(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name                   string
		studentID, companionID int64
		weekday                int
		wantErr                error
	}{
		{name: "self link", studentID: 12, companionID: 12, weekday: 1, wantErr: careplan.ErrCompanionSelfLink},
		{name: "weekday below range", studentID: 12, companionID: 13, weekday: 0, wantErr: careplan.ErrCompanionInvalidWeekday},
		{name: "weekend weekday", studentID: 12, companionID: 13, weekday: 6, wantErr: careplan.ErrCompanionInvalidWeekday},
		{name: "weekday above range", studentID: 12, companionID: 13, weekday: 9, wantErr: careplan.ErrCompanionInvalidWeekday},
		{name: "missing student", studentID: 0, companionID: 5, weekday: 1, wantErr: careplan.ErrCompanionStudentIDRequired},
		{name: "missing companion", studentID: 5, companionID: 0, weekday: 1, wantErr: careplan.ErrCompanionStudentIDRequired},
		{name: "negative id", studentID: -1, companionID: 5, weekday: 1, wantErr: careplan.ErrCompanionStudentIDRequired},
	} {
		t.Run(tt.name, func(t *testing.T) {
			edge, err := NewCompanionEdge(tt.studentID, tt.companionID, tt.weekday)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want errors.Is(..., %v)", err, tt.wantErr)
			}
			if edge != (careplan.CompanionEdge{}) {
				t.Errorf("expected a zero edge on error, got %+v", edge)
			}
		})
	}
}

// TestOtherCompanion checks the far-end lookup from both endpoints and from a
// child that is not part of the edge at all — the "ok" flag is what keeps
// unrelated edges out of a child's companion list.
func TestOtherCompanion(t *testing.T) {
	t.Parallel()

	edge, err := NewCompanionEdge(10, 20, 1)
	if err != nil {
		t.Fatalf("NewCompanionEdge() unexpected error: %v", err)
	}
	for _, tt := range []struct {
		name      string
		from      int64
		wantOther int64
		wantOK    bool
	}{
		{name: "from the low endpoint", from: 10, wantOther: 20, wantOK: true},
		{name: "from the high endpoint", from: 20, wantOther: 10, wantOK: true},
		{name: "from a non-member", from: 30, wantOther: 0, wantOK: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			other, ok := OtherCompanion(edge, tt.from)
			if other != tt.wantOther || ok != tt.wantOK {
				t.Errorf("OtherCompanion(%d) = %d, %v; want %d, %v", tt.from, other, ok, tt.wantOther, tt.wantOK)
			}
		})
	}
}

// TestCompanionWeekdayNumber pins the key<->number translation both API
// directions depend on.
func TestCompanionWeekdayNumber(t *testing.T) {
	t.Parallel()

	for key, want := range map[string]int{"mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5} {
		got, err := CompanionWeekdayNumber(key)
		if err != nil {
			t.Fatalf("CompanionWeekdayNumber(%q) unexpected error: %v", key, err)
		}
		if got != want {
			t.Errorf("CompanionWeekdayNumber(%q) = %d, want %d", key, got, want)
		}
		if back, ok := CompanionWeekdayKey(want); !ok || back != key {
			t.Errorf("CompanionWeekdayKey(%d) = %q, %v; want %q", want, back, ok, key)
		}
	}
	if _, err := CompanionWeekdayNumber("sat"); !errors.Is(err, careplan.ErrCompanionInvalidWeekday) {
		t.Errorf("CompanionWeekdayNumber(\"sat\") error = %v, want ErrCompanionInvalidWeekday", err)
	}
}
