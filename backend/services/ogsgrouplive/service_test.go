package ogsgrouplive

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
)

type failingBoolSettings struct{ err error }

func (s failingBoolSettings) ResolveBool(context.Context, string) (bool, error) {
	return false, s.err
}

func TestSortGroupsUsesGermanCollation(t *testing.T) {
	groups := []*educationModels.Group{{Name: "Zebra"}, {Name: "Äpfel"}}

	sortGroups(groups)

	assert.Equal(t, "Äpfel", groups[0].Name)
	assert.Equal(t, "Zebra", groups[1].Name)
}

func TestRoomStatusesOmitCurrentRoomWithoutGroupRoom(t *testing.T) {
	students := []Student{{ID: 101}}
	locations := &activeService.StudentLocationSnapshot{
		Visits: map[int64]*activeModels.Visit{101: {ActiveGroupID: 201}},
		Groups: map[int64]*activeModels.Group{201: {RoomID: 301}},
	}

	statuses := roomStatuses(students, locations, nil)
	status, ok := statuses["101"]

	require.True(t, ok)
	assert.False(t, status.InGroupRoom)
	assert.Nil(t, status.CurrentRoomID)
}

func TestResolvePhotosEnabledPropagatesSettingsFailure(t *testing.T) {
	want := errors.New("settings unavailable")

	enabled, err := resolvePhotosEnabled(context.Background(), failingBoolSettings{err: want})

	assert.False(t, enabled)
	assert.ErrorIs(t, err, want)
}

func TestNewServiceAppliesRuntimeDefaults(t *testing.T) {
	got, ok := NewService(Dependencies{}).(*service)

	require.True(t, ok)
	assert.NotNil(t, got.deps.Logger)
	assert.NotNil(t, got.deps.Now)
}

func TestValidateDependenciesRejectsIncompleteService(t *testing.T) {
	err := (&service{}).validateDependencies()

	assert.EqualError(t, err, "OGS group live service is not fully configured")
}

func TestProjectStudentsAppliesAccessAndPhotoRules(t *testing.T) {
	groupID := int64(12)
	photoPath := "/uploads/student-photos/avatar.jpg"
	sick, excused := true, true
	student := &userModels.Student{
		PersonID: 21, GroupID: &groupID, SchoolClass: "2a", PhotoPath: &photoPath,
		Sick: &sick, Excused: &excused,
	}
	student.ID = 11
	withoutPerson := &userModels.Student{PersonID: 99}
	withoutPerson.ID = 98
	data := &snapshot{
		persons: map[int64]*userModels.Person{
			21: {FirstName: "Ada", LastName: "Lovelace"},
		},
		locations: &activeService.StudentLocationSnapshot{
			Mode:        activeService.PresenceModeDetailed,
			Attendances: map[int64]*activeService.AttendanceStatus{},
			Visits:      map[int64]*activeModels.Visit{},
			Groups:      map[int64]*activeModels.Group{},
		},
	}

	got := projectStudents(
		[]*userModels.Student{student, withoutPerson},
		data,
		&userContextService.StudentAccessContext{IsAdmin: true},
		true,
	)

	require.Len(t, got, 1)
	assert.Equal(t, int64(11), got[0].ID)
	assert.Equal(t, "Ada", got[0].FirstName)
	assert.Equal(t, "Lovelace", got[0].LastName)
	assert.Equal(t, "2a", got[0].SchoolClass)
	assert.True(t, got[0].Sick)
	assert.True(t, got[0].Excused)
	assert.True(t, got[0].fullAccess)
	assert.Equal(t, "/api/students/11/photo/avatar.jpg", got[0].PhotoURL)
}

func TestBuildPhotoURLPreservesNonStoredURLs(t *testing.T) {
	assert.Empty(t, buildPhotoURL(11, ""))
	assert.Equal(t, "https://example.test/photo.jpg", buildPhotoURL(11, "https://example.test/photo.jpg"))
}

func TestApplyEffectiveStatusHonorsPrecedence(t *testing.T) {
	reportedAt := time.Date(2026, time.August, 1, 8, 30, 0, 0, time.UTC)

	tests := []struct {
		name   string
		start  Student
		status activeService.EffectiveStatus
		want   Student
	}{
		{
			name:   "sick clears lower-priority states",
			start:  Student{ClassTrip: true, Excused: true},
			status: activeService.EffectiveStatus{Sick: true, SickSince: &reportedAt},
			want:   Student{Sick: true, SickSince: &reportedAt},
		},
		{
			name:   "class trip clears lower-priority states",
			start:  Student{Sick: true, Excused: true},
			status: activeService.EffectiveStatus{ClassTrip: true, ClassTripSince: &reportedAt},
			want:   Student{ClassTrip: true, ClassTripSince: &reportedAt},
		},
		{
			name:   "excused applies when not sick",
			status: activeService.EffectiveStatus{Excused: true, ExcusedSince: &reportedAt},
			want:   Student{Excused: true, ExcusedSince: &reportedAt},
		},
		{
			name:   "excused does not override sickness",
			start:  Student{Sick: true},
			status: activeService.EffectiveStatus{Excused: true, ExcusedSince: &reportedAt},
			want:   Student{Sick: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.start
			applyEffectiveStatus(&got, tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPlanningLabelDescribesEveryPlanningReason(t *testing.T) {
	tests := []struct {
		decision scheduleService.DayPlanningDecision
		want     string
	}{
		{scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonUnplanned}, "ungeplant anwesend"},
		{scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonSick}, "krank gemeldet"},
		{scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonClassTrip}, "Klassenfahrt"},
		{scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonExcused}, "entschuldigt"},
		{scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonArrivalException, ComesToday: true}, "geplante Ankunft heute"},
		{scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonArrivalException, ExceptionNotes: "Arzttermin"}, "Arzttermin"},
		{scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonArrivalException}, "Tagesausnahme"},
		{scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonArrivalSchedule}, "Ankunftsplan heute"},
		{scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonPickupException, ComesToday: true}, "geplante Abholung heute"},
		{scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonPickupException, ExceptionNotes: "Familientag"}, "Familientag"},
		{scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonPickupSchedule}, "Abholplan heute"},
		{scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonTimetable}, "Betreuungsplan heute"},
		{scheduleService.DayPlanningDecision{Reason: scheduleService.DayPlanningReasonNoPlan}, "kein Plan für heute"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, planningLabel(tt.decision))
	}
}

func TestApplyTimesProjectsPlansNotesAndAttendance(t *testing.T) {
	arrivalTime := time.Date(2026, time.August, 1, 7, 45, 0, 0, time.UTC)
	checkIn := time.Date(2026, time.August, 1, 8, 10, 0, 0, time.UTC)
	checkOut := time.Date(2026, time.August, 1, 14, 5, 0, 0, time.UTC)
	student := Student{}

	applyTimes(
		&student,
		&scheduleService.EffectiveArrivalTime{
			ArrivalTime: &arrivalTime,
			IsException: true,
			Notes:       "später",
			DayNotes: []scheduleService.ArrivalNoteData{
				{Content: "Bus"},
				{Content: ""},
			},
		},
		&activeService.AttendanceStatus{CheckInTime: &checkIn, CheckOutTime: &checkOut},
	)

	require.NotNil(t, student.ArrivalTime)
	assert.Equal(t, "07:45", *student.ArrivalTime)
	assert.True(t, student.ArrivalIsException)
	assert.Equal(t, "später, Bus", student.ArrivalNotes)
	require.NotNil(t, student.ActualArrivalTime)
	require.NotNil(t, student.ActualPickupTime)
	assert.Equal(t, "10:10", *student.ActualArrivalTime)
	assert.Equal(t, "16:05", *student.ActualPickupTime)
}

func TestPickupTimesProjectsOnlyAccessibleScheduledStudents(t *testing.T) {
	pickupAt := time.Date(2026, time.August, 1, 15, 30, 0, 0, time.UTC)
	pickups := map[int64]*scheduleService.EffectivePickupTime{
		11: {
			Date: timezone.NewDate(2026, time.August, 1), PickupTime: &pickupAt,
			WeekdayName: "Samstag", IsException: true, Notes: "Oma",
			DayNotes: []scheduleService.NoteData{{ID: 7, Content: "Klingeln"}},
		},
	}

	got := pickupTimes(
		[]Student{{ID: 11, fullAccess: true}, {ID: 12, fullAccess: true}, {ID: 13}},
		pickups,
	)

	require.Len(t, got, 1)
	assert.Equal(t, int64(11), got[0].StudentID)
	assert.Equal(t, "2026-08-01", got[0].Date)
	require.NotNil(t, got[0].PickupTime)
	assert.Equal(t, "15:30", *got[0].PickupTime)
	assert.Equal(t, []DayNote{{ID: 7, Content: "Klingeln"}}, got[0].DayNotes)
}
