package slotlists

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

func TestIncludePickupParticipantUsesCareBoundaryAndActualPresence(t *testing.T) {
	t.Parallel()
	assert.False(t, includePickupParticipant(scheduleSvc.CareDayNotScheduled, false))
	assert.True(t, includePickupParticipant(scheduleSvc.CareDayNotScheduled, true))
	assert.True(t, includePickupParticipant(scheduleSvc.CareDayScheduled, false))
}

func ptrDate(d timezone.Date) *timezone.Date { return &d }

// TestEligibleOn_ImmediateActivation covers the #1565-review fix: an immediately
// activated child (enrollment.default_activation_mode = "immediate") is 'active'
// with a future enrolled_from, and must count from today — the active status
// overrides the enrolled_from lower bound. Actual attendance overrides every
// bound so safety information cannot disappear.
func TestEligibleOn_ImmediateActivation(t *testing.T) {
	t.Parallel()

	list := timezone.NewDate(2030, 9, 4)
	future := timezone.NewDate(2030, 12, 1)
	past := timezone.NewDate(2030, 8, 1)
	// The list is built for "today": the immediate-activation override lets an
	// active child count from today onward, so the requested date is today here.
	today := list

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
			name: "expired child without attendance is excluded",
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
			name: "inactive legacy child without enrollment bounds is excluded",
			student: &userModel.Student{
				Status: userModel.StudentStatusInactive,
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := eligibleOn(tc.student, list, today, false); got != tc.want {
				t.Fatalf("eligibleOn = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestEligibleOn_HistoricalDateBeforeEnrollment covers the #1565-review fix that
// immediate activation only overrides the enrolled_from lower bound from today
// onward: an active child must NOT be treated as retroactively enrolled for a
// past date before their enrollment began, or a stale/manual slot roster (and
// the /options counts) would show them as planned/missing before enrollment.
func TestEligibleOn_HistoricalDateBeforeEnrollment(t *testing.T) {
	t.Parallel()

	today := timezone.TodayDate()
	enrolledFrom := today.AddDays(-30) // enrollment started 30 days ago
	beforeEnrollment := today.AddDays(-60)
	withinWindow := today.AddDays(-10)

	activeChild := func(from timezone.Date) *userModel.Student {
		return &userModel.Student{
			Status:       userModel.StudentStatusActive,
			EnrolledFrom: ptrDate(from),
		}
	}

	if eligibleOn(activeChild(enrolledFrom), beforeEnrollment, today, false) {
		t.Fatal("active child must NOT be eligible for a past date before enrolled_from")
	}
	if !eligibleOn(activeChild(enrolledFrom), withinWindow, today, false) {
		t.Fatal("active child must be eligible within the enrollment window")
	}
	// Immediate activation is unchanged: a future enrolled_from must still let the
	// current day count.
	if !eligibleOn(activeChild(today.AddDays(30)), today, today, false) {
		t.Fatal("immediately-activated child must be eligible today despite a future enrolled_from")
	}
	if !eligibleOn(&userModel.Student{Status: userModel.StudentStatusInactive}, today, today, true) {
		t.Fatal("actual attendance must override the inactive marker")
	}
}

// TestSummarySlotCount_CancelledSlotsIgnored is a defensive backstop for the
// upstream cancellation filter: even a manually assembled Result containing a
// cancelled slot must not count it in the exported "Enthalten" summary.
func TestSummarySlotCount_CancelledSlots(t *testing.T) {
	t.Parallel()

	result := &Result{
		Slots: []Slot{
			{InstanceID: 1, Status: scheduleModel.InstanceStatusActive},    // baseline → counts
			{InstanceID: 2, Status: scheduleModel.InstanceStatusCancelled}, // always skipped
			{InstanceID: 3, Status: scheduleModel.InstanceStatusCancelled}, // always skipped
			{InstanceID: 4, Status: scheduleModel.InstanceStatusActive},    // deferred → skipped
		},
		deferredSlots: map[int64]struct{}{4: {}},
	}

	if got := summarySlotCount(Params{Target: TargetSlots}, result); got != 1 {
		t.Fatalf("summarySlotCount = %d, want 1 active non-deferred slot", got)
	}
}
