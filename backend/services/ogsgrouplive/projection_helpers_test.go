package ogsgrouplive

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

func TestPlanningLabelCoversEveryReason(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		decision scheduleService.DayPlanningDecision
		want     string
	}{
		{"unplanned", scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonUnplanned}, "ungeplant anwesend"},
		{"sick", scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonSick}, "krank gemeldet"},
		{"class trip", scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonClassTrip}, "Klassenfahrt"},
		{"excused", scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonExcused}, "entschuldigt"},
		{"arrival exception coming", scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonArrivalException, ComesToday: true}, "geplante Ankunft heute"},
		{"arrival exception absent with note", scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonArrivalException, ExceptionNotes: "Zahnarzt"}, "Zahnarzt"},
		{"arrival exception absent without note", scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonArrivalException}, "Tagesausnahme"},
		{"arrival schedule", scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonArrivalSchedule}, "Ankunftsplan heute"},
		{"pickup exception coming", scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonPickupException, ComesToday: true}, "geplante Abholung heute"},
		{"pickup exception absent", scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonPickupException}, "Tagesausnahme"},
		{"pickup schedule", scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonPickupSchedule}, "Abholplan heute"},
		{"timetable", scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonTimetable}, "Betreuungsplan heute"},
		{"no plan", scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonNoPlan}, "kein Plan für heute"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, planningLabel(tc.decision))
		})
	}
}

func TestApplyEffectiveStatusPrecedence(t *testing.T) {
	t.Parallel()

	since := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)

	t.Run("sick wins and clears the other flags", func(t *testing.T) {
		student := Student{Excused: true, ExcusedSince: &since, ClassTrip: true, ClassTripSince: &since}
		applyEffectiveStatus(&student, activeService.EffectiveStatus{Sick: true, SickSince: &since})
		assert.True(t, student.Sick)
		assert.Equal(t, &since, student.SickSince)
		assert.False(t, student.ClassTrip)
		assert.Nil(t, student.ClassTripSince)
		assert.False(t, student.Excused)
		assert.Nil(t, student.ExcusedSince)
	})

	t.Run("class trip wins over excused", func(t *testing.T) {
		student := Student{Sick: true, SickSince: &since}
		applyEffectiveStatus(&student, activeService.EffectiveStatus{ClassTrip: true, ClassTripSince: &since})
		assert.True(t, student.ClassTrip)
		assert.False(t, student.Sick)
		assert.Nil(t, student.SickSince)
		assert.False(t, student.Excused)
	})

	t.Run("excused applies only when not already sick", func(t *testing.T) {
		student := Student{}
		applyEffectiveStatus(&student, activeService.EffectiveStatus{Excused: true, ExcusedSince: &since})
		assert.True(t, student.Excused)
		assert.Equal(t, &since, student.ExcusedSince)

		sickStudent := Student{Sick: true}
		applyEffectiveStatus(&sickStudent, activeService.EffectiveStatus{Excused: true, ExcusedSince: &since})
		assert.False(t, sickStudent.Excused)
	})

	t.Run("empty status leaves the student untouched", func(t *testing.T) {
		student := Student{}
		applyEffectiveStatus(&student, activeService.EffectiveStatus{})
		assert.False(t, student.Sick)
		assert.False(t, student.ClassTrip)
		assert.False(t, student.Excused)
	})
}

func TestBuildPhotoURLRewritesOnlyStoredUploads(t *testing.T) {
	t.Parallel()

	assert.Empty(t, buildPhotoURL(5, ""))
	assert.Equal(t, "https://example.invalid/x.jpg", buildPhotoURL(5, "https://example.invalid/x.jpg"),
		"non-upload URLs pass through untouched")
	assert.Equal(t, "/api/students/5/photo/abc.jpg", buildPhotoURL(5, "/uploads/student-photos/abc.jpg"),
		"stored upload paths are rewritten to the authenticated proxy")
}

func TestApplyTimesFormatsArrivalAndAttendance(t *testing.T) {
	t.Parallel()

	arrivalAt := time.Date(2026, 8, 3, 13, 15, 0, 0, time.UTC)
	checkIn := time.Date(2026, 8, 3, 11, 2, 0, 0, time.UTC)
	checkOut := time.Date(2026, 8, 3, 14, 30, 0, 0, time.UTC)

	student := Student{}
	applyTimes(&student,
		&scheduleService.EffectiveArrivalTime{
			ArrivalTime: &arrivalAt,
			IsException: true,
			Notes:       "Ausnahme",
			DayNotes:    []scheduleService.ArrivalNoteData{{Content: "Bus fällt aus"}, {Content: ""}},
		},
		&activeService.AttendanceStatus{CheckInTime: &checkIn, CheckOutTime: &checkOut},
	)

	require.NotNil(t, student.ArrivalTime)
	assert.Equal(t, "13:15", *student.ArrivalTime)
	assert.True(t, student.ArrivalIsException)
	assert.Equal(t, "Ausnahme, Bus fällt aus", student.ArrivalNotes)
	require.NotNil(t, student.ActualArrivalTime)
	require.NotNil(t, student.ActualPickupTime)

	untouched := Student{}
	applyTimes(&untouched, nil, nil)
	assert.Nil(t, untouched.ArrivalTime)
	assert.Nil(t, untouched.ActualArrivalTime)
}

func TestPickupTimesSkipsRedactedAndMissingStudents(t *testing.T) {
	t.Parallel()

	pickupAt := time.Date(2026, 8, 3, 15, 0, 0, 0, time.UTC)
	students := []Student{
		{ID: 1, fullAccess: true},
		{ID: 2, fullAccess: false}, // redacted → skipped even with data
		{ID: 3, fullAccess: true},  // no pickup entry → skipped
	}
	pickups := map[int64]*scheduleService.EffectivePickupTime{
		1: {PickupTime: &pickupAt, IsException: true, Notes: "früher", DayNotes: []scheduleService.NoteData{{ID: 7, Content: "Oma holt ab"}}},
		2: {PickupTime: &pickupAt},
	}

	result := pickupTimes(students, pickups)

	require.Len(t, result, 1)
	assert.Equal(t, students[0].ID, result[0].StudentID)
	require.NotNil(t, result[0].PickupTime)
	assert.True(t, result[0].IsException)
	assert.Equal(t, "früher", result[0].Notes)
	require.Len(t, result[0].DayNotes, 1)
	assert.Equal(t, "Oma holt ab", result[0].DayNotes[0].Content)
}

func TestValidateDependenciesRejectsPartialWiring(t *testing.T) {
	t.Parallel()

	incomplete := &service{}
	assert.Error(t, incomplete.validateDependencies())
}

func TestSelectGroupFallsBackAndMatches(t *testing.T) {
	t.Parallel()

	first := &educationModels.Group{Name: "A"}
	first.ID = 1
	second := &educationModels.Group{Name: "B"}
	second.ID = 2
	groups := []*educationModels.Group{first, second}

	assert.Same(t, first, selectGroup(groups, 0), "no request resolves the first sorted group")
	assert.Same(t, second, selectGroup(groups, 2))
	assert.Nil(t, selectGroup(groups, 99), "unsupervised group must not resolve")
}

// applyPlanning attaches the guardian's pending note regardless of read scope:
// in open care the staffer who decides the request supervises no group, so the
// child reaches them redacted (#2232). The day plan itself stays full-access.
func TestApplyPlanningAttachesPendingNoteToRedactedStudent(t *testing.T) {
	t.Parallel()

	state := &buildState{
		projected: []Student{
			{ID: 11, fullAccess: true},
			{ID: 12, fullAccess: false},
		},
		data: &snapshot{locations: &activeService.StudentLocationSnapshot{
			Attendances: map[int64]*activeService.AttendanceStatus{},
		}},
		arrivals:  map[int64]*scheduleService.EffectiveArrivalTime{},
		pickups:   map[int64]*scheduleService.EffectivePickupTime{},
		timetable: map[int64]struct{}{},
	}
	pending := map[int64]*activeModels.ExcusedAbsenceRequest{
		11: {Note: "Zahnarzt"},
		12: {Note: "Arzttermin"},
	}

	applyPlanning(state, pending)

	require.NotNil(t, state.projected[0].PendingExcusedNote)
	assert.Equal(t, "Zahnarzt", *state.projected[0].PendingExcusedNote)
	require.NotNil(t, state.projected[1].PendingExcusedNote, "redacted child must still carry the note its reviewer owns")
	assert.Equal(t, "Arzttermin", *state.projected[1].PendingExcusedNote)

	assert.NotEmpty(t, state.projected[0].DayPlanningStatus)
	assert.Empty(t, state.projected[1].DayPlanningStatus, "day plan stays gated on full access")
}
