package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

func TestIncludePickupParticipantUsesCareBoundaryAndActualPresence(t *testing.T) {
	t.Parallel()
	assert.False(t, includePickupParticipant(ports.CareDayNotScheduled, false))
	assert.True(t, includePickupParticipant(ports.CareDayNotScheduled, true))
	assert.True(t, includePickupParticipant(ports.CareDayScheduled, false))
}

// recordingRules is the enrollment rule as the application sees it: the
// binding decides, the application only routes the facts to it.
type recordingRules struct {
	enrolled bool
	facts    ports.StudentFacts
}

func (r *recordingRules) RowCareDay(bool, ports.RosterFacts, ports.CareDay) ports.CareDay {
	return ports.CareDayUnknown
}

func (r *recordingRules) EnrolledOn(student ports.StudentFacts, _, _ timezone.Date) bool {
	r.facts = student
	return r.enrolled
}

// TestEligibleOn_CandidateRulesBeforeEnrollment covers the candidate rules the
// projection decides itself: documented presence wins before any enrollment
// bound, a graduate is never a candidate, and every other child is handed to
// the owner's enrollment rule with its parsed enrolment interval. The interval
// semantics themselves are pinned in the compose binding test.
func TestEligibleOn_CandidateRulesBeforeEnrollment(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2030, 9, 4)

	t.Run("nil student is never eligible", func(t *testing.T) {
		rules := &recordingRules{enrolled: true}
		ok, err := eligibleOn(rules, nil, date, date, false)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("actual attendance overrides the inactive marker without asking the rule", func(t *testing.T) {
		rules := &recordingRules{enrolled: false}
		ok, err := eligibleOn(rules, &peopledirectory.Student{Status: "inactive"}, date, date, true)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("a graduate is excluded before the enrollment rule", func(t *testing.T) {
		rules := &recordingRules{enrolled: true}
		ok, err := eligibleOn(rules, &peopledirectory.Student{Status: peopledirectory.StudentStatusAlumnus}, date, date, false)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("the enrolment interval reaches the rule as calendar days", func(t *testing.T) {
		rules := &recordingRules{enrolled: true}
		ok, err := eligibleOn(rules, &peopledirectory.Student{
			Status: "active", EnrolledFrom: "2030-08-01", EnrolledUntil: "2031-07-31",
		}, date, date, false)
		require.NoError(t, err)
		assert.True(t, ok)
		require.NotNil(t, rules.facts.EnrolledFrom)
		require.NotNil(t, rules.facts.EnrolledUntil)
		assert.Equal(t, timezone.NewDate(2030, 8, 1), *rules.facts.EnrolledFrom)
		assert.Equal(t, timezone.NewDate(2031, 7, 31), *rules.facts.EnrolledUntil)
		assert.Equal(t, "active", rules.facts.Status)
	})

	t.Run("an unparsable enrolment date is a data error, never silently unset", func(t *testing.T) {
		rules := &recordingRules{enrolled: true}
		_, err := eligibleOn(rules, &peopledirectory.Student{ID: 7, Status: "active", EnrolledFrom: "01.08.2030"}, date, date, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "student 7")
	})
}

// TestSummarySlotCount_CancelledSlotsIgnored is a defensive backstop for the
// upstream cancellation filter: even a manually assembled Result containing a
// cancelled slot must not count it in the exported "Enthalten" summary.
func TestSummarySlotCount_CancelledSlots(t *testing.T) {
	t.Parallel()

	result := &classday.Result{
		Slots: []classday.Slot{
			{InstanceID: 1, Status: timetable.InstanceStatusActive},    // baseline → counts
			{InstanceID: 2, Status: timetable.InstanceStatusCancelled}, // always skipped
			{InstanceID: 3, Status: timetable.InstanceStatusCancelled}, // always skipped
			{InstanceID: 4, Status: timetable.InstanceStatusActive},    // deferred → skipped
		},
		DeferredSlots: map[int64]struct{}{4: {}},
	}

	if got := summarySlotCount(listRequest{Params: classday.Params{Target: classday.TargetSlots}}, result); got != 1 {
		t.Fatalf("summarySlotCount = %d, want 1 active non-deferred slot", got)
	}
}
