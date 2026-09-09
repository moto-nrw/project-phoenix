package compose_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/classday/compose"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
)

func ptrDate(d timezone.Date) *timezone.Date { return &d }

// TestRulesEnrolledOn_ImmediateActivation covers the #1565-review fix through
// the binding: an immediately activated child (enrollment.default_activation_mode
// = "immediate") is 'active' with a future enrolled_from, and must count from
// today — the active status overrides the enrolled_from lower bound.
func TestRulesEnrolledOn_ImmediateActivation(t *testing.T) {
	t.Parallel()

	list := timezone.NewDate(2030, 9, 4)
	future := timezone.NewDate(2030, 12, 1)
	past := timezone.NewDate(2030, 8, 1)
	// The list is built for "today": the immediate-activation override lets an
	// active child count from today onward, so the requested date is today here.
	today := list

	cases := []struct {
		name    string
		student ports.StudentFacts
		want    bool
	}{
		{
			name:    "active child with future enrolled_from is eligible now (immediate activation)",
			student: ports.StudentFacts{Status: string(userModels.StudentStatusActive), EnrolledFrom: ptrDate(future)},
			want:    true,
		},
		{
			name:    "expired child without attendance is excluded",
			student: ports.StudentFacts{Status: string(userModels.StudentStatusActive), EnrolledUntil: ptrDate(past)},
			want:    false,
		},
		{
			name:    "pending child before enrolled_from is excluded (interval is source of truth)",
			student: ports.StudentFacts{Status: string(userModels.StudentStatusPending), EnrolledFrom: ptrDate(future)},
			want:    false,
		},
		{
			name:    "pending child already enrolled on the date is included",
			student: ports.StudentFacts{Status: string(userModels.StudentStatusPending), EnrolledFrom: ptrDate(past)},
			want:    true,
		},
		{
			name:    "inactive legacy child without enrollment bounds is excluded",
			student: ports.StudentFacts{Status: string(userModels.StudentStatusInactive)},
			want:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (compose.Rules{}).EnrolledOn(tc.student, list, today); got != tc.want {
				t.Fatalf("EnrolledOn = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestRulesEnrolledOn_HistoricalDateBeforeEnrollment covers the #1565-review
// fix that immediate activation only overrides the enrolled_from lower bound
// from today onward: an active child must NOT be treated as retroactively
// enrolled for a past date before their enrollment began.
func TestRulesEnrolledOn_HistoricalDateBeforeEnrollment(t *testing.T) {
	t.Parallel()

	today := timezone.NewDate(2030, 9, 4)
	enrolledFrom := today.AddDays(-30) // enrollment started 30 days ago
	beforeEnrollment := today.AddDays(-60)
	withinWindow := today.AddDays(-10)

	activeChild := func(from timezone.Date) ports.StudentFacts {
		return ports.StudentFacts{Status: string(userModels.StudentStatusActive), EnrolledFrom: ptrDate(from)}
	}
	rules := compose.Rules{}

	if rules.EnrolledOn(activeChild(enrolledFrom), beforeEnrollment, today) {
		t.Fatal("active child must NOT be eligible for a past date before enrolled_from")
	}
	if !rules.EnrolledOn(activeChild(enrolledFrom), withinWindow, today) {
		t.Fatal("active child must be eligible within the enrollment window")
	}
	// Immediate activation is unchanged: a future enrolled_from must still let the
	// current day count.
	if !rules.EnrolledOn(activeChild(today.AddDays(30)), today, today) {
		t.Fatal("immediately-activated child must be eligible today despite a future enrolled_from")
	}
}

// TestRulesRowCareDay pins the binding to the owner's derivation: a manual
// status always answers unknown, a frozen not_scheduled marker on a completed
// block stays a non-booking, and an expected row on a live block carries the
// plan verdict through.
func TestRulesRowCareDay(t *testing.T) {
	t.Parallel()
	rules := compose.Rules{}
	now := timezone.NewDate(2030, 9, 4).BerlinMidnight()

	if got := rules.RowCareDay(false, ports.RosterFacts{Status: "expected", ManualStatusAt: &now}, ports.CareDayNotScheduled); got != ports.CareDayUnknown {
		t.Fatalf("manual status must answer unknown, got %s", got)
	}
	if got := rules.RowCareDay(true, ports.RosterFacts{Status: "expected", NotScheduled: true}, ports.CareDayScheduled); got != ports.CareDayNotScheduled {
		t.Fatalf("frozen marker on a completed block must stay not_scheduled, got %s", got)
	}
	if got := rules.RowCareDay(false, ports.RosterFacts{Status: "expected"}, ports.CareDayCancelled); got != ports.CareDayCancelled {
		t.Fatalf("expected row on a live block must carry the plan verdict, got %s", got)
	}
	id := new(int64)
	if got := rules.RowCareDay(false, ports.RosterFacts{Status: "absent", StudentStatusDayID: id}, ports.CareDayNotScheduled); got != ports.CareDayNotScheduled {
		t.Fatalf("plan-owned absence on a not-scheduled day must stay a non-booking, got %s", got)
	}
}
