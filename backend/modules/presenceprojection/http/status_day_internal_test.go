package presenceprojectionhttp

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusDayOverviewRangeUsesSingleTodaySnapshot(t *testing.T) {
	t.Parallel()

	today := calendar.NewDate(2026, 5, 25)
	from, to, err := parseStatusDayOverviewRange(
		httptest.NewRequest("GET", "/status-days", nil),
		today,
	)

	require.NoError(t, err)
	assert.Equal(t, today, from)
	assert.Equal(t, calendar.NewDate(2026, 7, 25), to)
}

func TestStudentStatusDayResponsesAndEffectiveStatus(t *testing.T) {
	t.Parallel()

	reportedSick := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)
	reportedExcused := reportedSick.Add(time.Hour)
	sick := &absencerecords.StudentStatusDay{
		ID:         42,
		StudentID:  90,
		Date:       calendar.NewDate(2026, 5, 25),
		Status:     absencerecords.StudentStatusDaySick,
		ReportedAt: reportedSick,
		Source:     absencerecords.StudentStatusSourcePlanned,
	}
	excused := &absencerecords.StudentStatusDay{
		ID:         43,
		StudentID:  91,
		Date:       calendar.NewDate(2026, 5, 26),
		Status:     absencerecords.StudentStatusDayExcused,
		ReportedAt: reportedExcused,
		Source:     absencerecords.StudentStatusSourceManual,
	}

	note := "Fieber"
	sick.Note = &note
	responses := studentpresence.NewStudentStatusDayResponses([]*absencerecords.StudentStatusDay{sick, excused})
	require.Len(t, responses, 2)
	assert.Equal(t, "Krank", responses[0].Label)
	assert.Equal(t, "2026-05-25", responses[0].Date)
	assert.Equal(t, "Entschuldigt", responses[1].Label)
	require.NotNil(t, responses[0].Note)
	assert.Equal(t, "Fieber", *responses[0].Note)

	conflictResponses := studentpresence.NewStudentStatusDayConflictResponses([]*absencerecords.StudentStatusDay{sick})
	require.Len(t, conflictResponses, 1)
	assert.Equal(t, sick.ID, conflictResponses[0].ID)
	assert.Nil(t, conflictResponses[0].Note, "409 conflict payloads must omit free-text notes")

	studentResponses := []StudentResponse{{ID: 90}, {ID: 91}}
	applyEffectiveStatusDaysToResponses(studentResponses, []*absencerecords.StudentStatusDay{excused, sick})
	assert.True(t, studentResponses[0].Sick)
	assert.False(t, studentResponses[0].Excused)
	assert.True(t, studentResponses[1].Excused)

	classTrip := &absencerecords.StudentStatusDay{
		ID:         44,
		StudentID:  92,
		Date:       calendar.NewDate(2026, 5, 27),
		Status:     absencerecords.StudentStatusDayClassTrip,
		ReportedAt: reportedExcused.Add(time.Hour),
		Source:     absencerecords.StudentStatusSourcePlanned,
	}
	classTripResponse := StudentResponse{ID: 92, Excused: true, Sick: true}
	applyEffectiveStatusDays(&classTripResponse, []*absencerecords.StudentStatusDay{classTrip})
	assert.True(t, classTripResponse.ClassTrip)
	assert.False(t, classTripResponse.Sick)
	assert.False(t, classTripResponse.Excused)

	alreadySick := StudentResponse{ID: 91, Sick: true}
	applyEffectiveStatusDays(&alreadySick, []*absencerecords.StudentStatusDay{excused})
	assert.True(t, alreadySick.Sick)
	assert.False(t, alreadySick.Excused)

	applyEffectiveStatusDays(nil, []*absencerecords.StudentStatusDay{sick})
	applyEffectiveStatusDays(&alreadySick, nil)
	applyEffectiveStatusDaysToResponses(nil, []*absencerecords.StudentStatusDay{sick})
	applyEffectiveStatusDaysToResponses([]StudentResponse{{ID: 92}}, nil)
}

func TestApplyStatusDaysForDateUsesRequestedDate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	repo := &StatusDayRepositoryStub{
		findByIDsRows: []*absencerecords.StudentStatusDay{
			{
				ID:         42,
				StudentID:  90,
				Date:       calendar.DateFromTime(now),
				Status:     absencerecords.StudentStatusDaySick,
				ReportedAt: now,
				Source:     absencerecords.StudentStatusSourcePlanned,
			},
		},
	}
	resource := testResource(ResourceConfig{
		StudentStatusDayService: ComposeStatusDays(repo), Logger: slog.Default()})
	responses := []StudentResponse{{ID: 90}, {ID: 91}}

	require.NoError(t, resource.applyStatusDaysForDate(testpkg.WithPermissionsContext(context.Background(), []string{"users:read"}), responses, now))

	assert.Equal(t, calendar.DateFromTime(now), repo.findByIDsDate)
	assert.Equal(t, []int64{90, 91}, repo.findByIDsStudentIDs)
	assert.True(t, responses[0].Sick)
	assert.False(t, responses[0].Excused)
	assert.False(t, responses[1].Sick)
	assert.False(t, responses[1].Excused)
}

func TestResolveDayPlanningStatusDaysOverridePlans(t *testing.T) {
	t.Parallel()

	arrivalTime := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)
	pickupTime := time.Date(2026, 5, 25, 15, 30, 0, 0, time.UTC)
	timetableIDs := map[int64]struct{}{90: {}}

	status, reason, _ := resolveDayPlanningForDate(
		StudentResponse{ID: 90, Sick: true},
		&careplan.EffectiveArrivalTime{ArrivalTime: &arrivalTime},
		&careplan.EffectivePickupTime{PickupTime: &pickupTime},
		nil,
		timetableIDs,
		true,
	)

	assert.Equal(t, DayPlanningStatusNotComingToday, status)
	assert.Equal(t, dayPlanningReasonSick, reason)
}

func TestResolveDayPlanningActualAttendanceOverridesStatusDays(t *testing.T) {
	t.Parallel()

	checkInTime := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)

	status, reason, _ := resolveDayPlanningForDate(
		StudentResponse{ID: 90, Sick: true},
		nil,
		nil,
		&studentpresence.DailyAttendanceStatus{Status: "checked_in", CheckInTime: &checkInTime},
		map[int64]struct{}{},
		true,
	)

	assert.Equal(t, DayPlanningStatusComesToday, status)
	assert.Equal(t, dayPlanningReasonUnplanned, reason)
}

type StatusDayRepositoryStub struct {
	upsertErr           error
	clearForDatesErr    error
	clearByIDErr        error
	findByIDRow         *absencerecords.StudentStatusDay
	findByIDErr         error
	findRangeErr        error
	findRangeRows       []*absencerecords.StudentStatusDay
	findByIDsRows       []*absencerecords.StudentStatusDay
	findByIDsDate       calendar.Date
	findByIDsStudentIDs []int64
}

func (r *StatusDayRepositoryStub) UpsertReported(_ context.Context, _ *absencerecords.StudentStatusDay) error {
	return r.upsertErr
}

func (r *StatusDayRepositoryStub) MarkCleared(_ context.Context, _ int64, _ string, _ calendar.Date, _ time.Time, _ string) error {
	return nil
}

func (r *StatusDayRepositoryStub) MarkClearedByID(_ context.Context, _ int64, _ time.Time, _ string) error {
	return r.clearByIDErr
}

func (r *StatusDayRepositoryStub) MarkClearedForDates(_ context.Context, _ int64, _ string, _ []calendar.Date, _ time.Time, _ string) error {
	return r.clearForDatesErr
}

func (r *StatusDayRepositoryStub) FindActiveByID(_ context.Context, _ int64) (*absencerecords.StudentStatusDay, error) {
	if r.findByIDErr != nil {
		return nil, r.findByIDErr
	}
	if r.findByIDRow != nil {
		return r.findByIDRow, nil
	}
	return nil, errFakeRowNotFound
}

func (r *StatusDayRepositoryStub) FindActiveByStudentAndDateRange(_ context.Context, studentID int64, startDate, _ calendar.Date) ([]*absencerecords.StudentStatusDay, error) {
	if r.findRangeErr != nil {
		return nil, r.findRangeErr
	}
	if r.findRangeRows != nil {
		return r.findRangeRows, nil
	}
	return []*absencerecords.StudentStatusDay{
		{
			ID:         42,
			StudentID:  studentID,
			Date:       startDate,
			Status:     absencerecords.StudentStatusDaySick,
			ReportedAt: time.Now(),
			Source:     absencerecords.StudentStatusSourcePlanned,
		},
	}, nil
}

func (r *StatusDayRepositoryStub) FindActiveByStudentIDsAndDate(_ context.Context, studentIDs []int64, date calendar.Date) ([]*absencerecords.StudentStatusDay, error) {
	r.findByIDsStudentIDs = append([]int64(nil), studentIDs...)
	r.findByIDsDate = date
	return r.findByIDsRows, nil
}

func (r *StatusDayRepositoryStub) FindSignedOffByStudentIDsAndDate(_ context.Context, studentIDs []int64, date calendar.Date) ([]*absencerecords.StudentStatusDay, error) {
	r.findByIDsStudentIDs = append([]int64(nil), studentIDs...)
	r.findByIDsDate = date
	return r.findByIDsRows, nil
}

func (r *StatusDayRepositoryStub) FindByStudentAndDateRange(_ context.Context, _ int64, _, _ calendar.Date) ([]*absencerecords.StudentStatusDay, error) {
	return nil, nil
}

// Stubs for the issue #585 refactor interface additions — unused here.
func (r *StatusDayRepositoryStub) ArchiveAndClearStatusFlag(context.Context, string, string, string, calendar.Date, time.Time, string) (int64, error) {
	return 0, nil
}

func (r *StatusDayRepositoryStub) CountEffectiveDashboardAbsences(context.Context, calendar.Date) (*absencerecords.StudentStatusCounts, error) {
	return &absencerecords.StudentStatusCounts{}, nil
}

// errFakeRowNotFound is a repository not-found in the shape the routes and
// services recognise (the RepositoryNotFound marker of models/base).
var errFakeRowNotFound error = fakeRowNotFoundError{}

type fakeRowNotFoundError struct{}

func (fakeRowNotFoundError) Error() string { return "record not found" }

// RepositoryNotFound marks the repository not-found sentinel.
func (fakeRowNotFoundError) RepositoryNotFound() {}

var ComposeStatusDays func(*StatusDayRepositoryStub) studentpresence.StatusDays
