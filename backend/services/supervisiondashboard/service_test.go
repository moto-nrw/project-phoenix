package supervisiondashboard

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	facilitiesService "github.com/moto-nrw/project-phoenix/services/facilities"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

func TestSelectGroup(t *testing.T) {
	groups := []Group{{ID: 10}, {ID: 20}}
	yardGroupID := int64(30)

	tests := []struct {
		name      string
		requested int64
		yard      *facilitiesService.SchulhofStatus
		want      *int64
		wantErr   error
	}{
		{name: "defaults to the first group", want: int64Ptr(10)},
		{name: "uses a requested group", requested: 20, want: int64Ptr(20)},
		{name: "permits the supervised yard group", requested: 30, yard: &facilitiesService.SchulhofStatus{IsUserSupervising: true, ActiveGroupID: &yardGroupID}, want: int64Ptr(30)},
		{name: "rejects an unsupervised group", requested: 40, wantErr: ErrForbiddenGroup},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectGroup(groups, tt.yard, tt.requested)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}

	got, err := selectGroup(nil, nil, 0)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestBuildVisitsAndEffectiveTimes(t *testing.T) {
	checkedIn := time.Date(2026, time.August, 19, 10, 0, 0, 0, time.UTC)
	checkedOut := checkedIn.Add(time.Hour)
	sick, excused := true, true
	storedPhoto := studentPhotoStoredURLPrefix + "portrait.jpg"
	rows := []*activeModels.VisitWithStudentDisplay{{
		StudentID: 1, ActiveGroupID: 2, EntryTime: checkedIn,
		FirstName: "  Erika", LastName: "Mustermann ", SchoolClass: "4a", OGSGroupName: "Bären",
		Sick: &sick, SickSince: &checkedIn, Excused: &excused, ExcusedSince: &checkedOut, PhotoPath: &storedPhoto,
	}}

	visits := buildVisits(rows, map[int64]*activeService.AttendanceStatus{1: {CheckInTime: &checkedIn, CheckOutTime: &checkedOut}}, true, true)
	require.Len(t, visits, 1)
	assert.Equal(t, "Erika Mustermann", visits[0].StudentName)
	assert.True(t, visits[0].Sick)
	assert.True(t, visits[0].Excused)
	assert.Equal(t, "/api/students/1/photo/portrait.jpg", visits[0].PhotoURL)
	assert.Equal(t, timezone.FormatBerlinClock(&checkedIn), visits[0].ActualArrivalTime)
	assert.Equal(t, timezone.FormatBerlinClock(&checkedOut), visits[0].ActualPickupTime)

	privateVisits := buildVisits(rows, nil, false, true)
	assert.Nil(t, privateVisits[0].ActualArrivalTime)
	assert.Empty(t, privateVisits[0].PhotoURL)

	pickup := time.Date(2026, time.August, 19, 15, 30, 0, 0, time.UTC)
	arrival := time.Date(2026, time.August, 19, 11, 45, 0, 0, time.UTC)
	pickupItem := pickupTimeFromEffective(1, &scheduleService.EffectivePickupTime{
		Date: timezone.DateFromTime(checkedIn), PickupTime: &pickup, WeekdayName: "Mittwoch", IsException: true, Notes: "Oma", DayNotes: []scheduleService.NoteData{{ID: 3, Content: "Klingeln"}},
	})
	assert.Equal(t, "15:30", *pickupItem.PickupTime)
	assert.Equal(t, []DayNote{{ID: 3, Content: "Klingeln"}}, pickupItem.DayNotes)

	arrivalItem := arrivalTimeFromEffective(1, &scheduleService.EffectiveArrivalTime{
		Date: timezone.DateFromTime(checkedIn), ArrivalTime: &arrival, WeekdayName: "Mittwoch", Notes: "Bus", DayNotes: []scheduleService.ArrivalNoteData{{ID: 4, Content: "Verspätung"}},
	})
	assert.Equal(t, "11:45", *arrivalItem.ExpectedArrival)
	assert.Equal(t, []DayNote{{ID: 4, Content: "Verspätung"}}, arrivalItem.DayNotes)
}

func TestBuildPhotoURL(t *testing.T) {
	assert.Empty(t, buildPhotoURL(1, ""))
	assert.Equal(t, "https://example.test/photo.jpg", buildPhotoURL(1, "https://example.test/photo.jpg"))
}

func int64Ptr(value int64) *int64 { return &value }
