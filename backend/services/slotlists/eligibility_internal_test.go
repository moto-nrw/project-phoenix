package slotlists

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
)

func ptrDate(d timezone.Date) *timezone.Date { return &d }

// TestEligibleOn_ImmediateActivation covers the #1565-review fix: an immediately
// activated child (enrollment.default_activation_mode = "immediate") is 'active'
// with a future enrolled_from, and must count from today — the active status
// overrides the enrolled_from lower bound. enrolled_until still bounds the top
// end for every status, and non-active children keep the interval semantics.
func TestEligibleOn_ImmediateActivation(t *testing.T) {
	list := timezone.NewDate(2030, 9, 4)
	future := timezone.NewDate(2030, 12, 1)
	past := timezone.NewDate(2030, 8, 1)

	cases := []struct {
		name    string
		student *userModel.Student
		want    bool
	}{
		{
			name:    "nil student is never eligible",
			student: nil,
			want:    false,
		},
		{
			name: "active child with future enrolled_from is eligible now (immediate activation)",
			student: &userModel.Student{
				Status:       userModel.StudentStatusActive,
				EnrolledFrom: ptrDate(future),
			},
			want: true,
		},
		{
			name: "active child is still bounded by enrolled_until",
			student: &userModel.Student{
				Status:        userModel.StudentStatusActive,
				EnrolledUntil: ptrDate(past),
			},
			want: false,
		},
		{
			name: "pending child before enrolled_from is excluded (interval is source of truth)",
			student: &userModel.Student{
				Status:       userModel.StudentStatusPending,
				EnrolledFrom: ptrDate(future),
			},
			want: false,
		},
		{
			name: "pending child already enrolled on the date is included",
			student: &userModel.Student{
				Status:       userModel.StudentStatusPending,
				EnrolledFrom: ptrDate(past),
			},
			want: true,
		},
		{
			name: "no interval falls back to status: inactive is excluded",
			student: &userModel.Student{
				Status: userModel.StudentStatusInactive,
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := eligibleOn(tc.student, list); got != tc.want {
				t.Fatalf("eligibleOn = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSummarySlotCount_CancelledSlots covers the #1565-review fix that a slot
// cancelled AFTER it ran (and so retaining present rows) counts as a contained
// Termin, while a cancelled slot that never ran is still skipped. Deferred slots
// are excluded regardless. The active slot is the baseline that always counts.
func TestSummarySlotCount_CancelledSlots(t *testing.T) {
	result := &Result{
		Slots: []Slot{
			{InstanceID: 1, Status: scheduleModel.InstanceStatusActive},    // baseline → counts
			{InstanceID: 2, Status: scheduleModel.InstanceStatusCancelled}, // never ran → skipped
			{InstanceID: 3, Status: scheduleModel.InstanceStatusCancelled}, // ran, retained rows → counts
			{InstanceID: 4, Status: scheduleModel.InstanceStatusActive},    // deferred → skipped
		},
		retainedCancelledSlots: map[int64]struct{}{3: {}},
		deferredSlots:          map[int64]struct{}{4: {}},
	}

	if got := summarySlotCount(Params{Target: TargetSlots}, result); got != 2 {
		t.Fatalf("summarySlotCount = %d, want 2 (active baseline + ran-then-cancelled)", got)
	}
}
