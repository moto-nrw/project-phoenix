package contracttest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type emptyPickupBookings struct{}

func (emptyPickupBookings) ListApprovedByStudentIDsInRange(context.Context, []int64, calendar.Date, calendar.Date) ([]*careplan.ApprovedBooking, error) {
	return nil, nil
}

type pickupBaselineReadProbe struct {
	compose.PickupBaselineRecords
	scheduleReads int
	offeringReads int
}

func (p *pickupBaselineReadProbe) ListPickupSchedules(ctx context.Context, filter careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, error) {
	p.scheduleReads++
	return p.PickupBaselineRecords.ListPickupSchedules(ctx, filter)
}

func (p *pickupBaselineReadProbe) ListCareOfferings(context.Context, careplan.CareOfferingFilter) ([]careplan.CareOffering, error) {
	p.offeringReads++
	return nil, errors.New("unexpected offering catalog read without booking IDs")
}

func TestNativePickupBaselinesAvoidUnfilteredReadsAndFailClosedOnSettings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Baseline", "Native", "1a")
	records := &pickupBaselineReadProbe{PickupBaselineRecords: careplantest.NewCarePlan(t, db)}
	var modeErr error
	service, err := compose.NewPickupBaselines(records, emptyPickupBookings{}, func(context.Context) (bool, error) { return true, modeErr })
	require.NoError(t, err)
	date := calendar.TodayDate()
	_, err = service.Project(ctx, nil, date, date)
	require.NoError(t, err)
	require.Zero(t, records.scheduleReads)
	projection, err := service.Project(ctx, []int64{student.ID}, date, date)
	require.NoError(t, err)
	require.True(t, projection.BookingsAuthoritative)
	require.Equal(t, 1, records.scheduleReads)
	require.Zero(t, records.offeringReads)

	modeErr = errors.New("tenant setting unavailable")
	_, err = service.Project(ctx, []int64{student.ID}, date, date)
	require.ErrorIs(t, err, modeErr)
	require.ErrorContains(t, err, "resolve booking mode")
	require.Equal(t, 1, records.scheduleReads, "failed settings must not fall back to stored schedules")
}

func TestNativePickupSchedulesPersistAndRejectDuplicateStaffEdits(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	owner := careplantest.NewCarePlan(t, db)
	student := testpkg.CreateTestStudent(t, db, "Pickup", "Native", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Pickup", "Staff")
	service, err := compose.NewPickupSchedules(db, owner, excusalBaseline{}, nil, nil, nil)
	require.NoError(t, err)
	clock := time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC)
	weekly := &careplan.PickupSchedule{StudentID: student.ID, Weekday: 1, PickupTime: clock, CreatedBy: staff.ID}
	require.NoError(t, service.UpsertStudentPickupSchedule(ctx, weekly))
	require.Positive(t, weekly.ID)
	require.Equal(t, testpkg.Tenant(t), weekly.TenantID)
	stored, err := owner.FindPickupSchedule(ctx, weekly.ID)
	require.NoError(t, err)
	require.Equal(t, "15:00", stored.PickupTime.Format("15:04"))

	date := calendar.TodayDate().AddDays(7)
	note := &careplan.PickupNote{StudentID: student.ID, NoteDate: careplan.Date(date), Content: "Native note", CreatedBy: staff.ID}
	require.NoError(t, service.CreateStudentPickupNote(ctx, note))
	notes, err := service.GetStudentPickupNotesForDate(ctx, student.ID, date)
	require.NoError(t, err)
	require.Len(t, notes, 1)
	require.Equal(t, note.ID, notes[0].ID)

	exception, err := service.CreateOrReclaimException(ctx, student.ID, date, &clock, nil, staff.ID, nil)
	require.NoError(t, err)
	require.Positive(t, exception.ID)
	require.Equal(t, testpkg.Tenant(t), exception.TenantID)
	later := clock.Add(time.Hour)
	_, err = service.CreateOrReclaimException(ctx, student.ID, date, &later, nil, staff.ID, nil)
	require.ErrorIs(t, err, careplan.ErrCareExceptionDayConflict)
	reclaimed, err := service.UpdateException(ctx, exception.ID, student.ID, date, nil, &later, false, nil)
	require.NoError(t, err)
	require.Equal(t, exception.ID, reclaimed.ID)
	require.Equal(t, "16:00", reclaimed.PickupTime.Format("15:04"))
	other := testpkg.CreateTestStudent(t, db, "Other", "Pickup", "1a")
	require.ErrorIs(t, service.DeleteStudentPickupException(ctx, reclaimed.ID, other.ID), careplan.ErrCareExceptionWrongStudent)
	preserved, err := service.GetStudentPickupExceptionByID(ctx, reclaimed.ID)
	require.NoError(t, err)
	require.Equal(t, student.ID, preserved.StudentID)
	require.NoError(t, service.DeleteStudentPickupException(ctx, reclaimed.ID, reclaimed.StudentID))
	missing, err := service.GetStudentPickupExceptionForDate(ctx, student.ID, date)
	require.NoError(t, err)
	require.Nil(t, missing)
	require.NoError(t, service.DeleteAllStudentPickupNotes(ctx, student.ID))
	require.NoError(t, service.DeleteStudentPickupSchedule(ctx, weekly.ID))
}

func TestNativePickupApprovalRecordsPreserveDayAndConflict(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	owner := careplantest.NewCarePlan(t, db)
	student := testpkg.CreateTestStudent(t, db, "Approval", "Records", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Approval", "Writer")
	records, err := compose.NewPickupApprovalExceptions(owner)
	require.NoError(t, err)
	date := calendar.TodayDate().AddDays(7)
	missing, err := records.FindForDate(ctx, student.ID, date)
	require.NoError(t, err)
	require.Nil(t, missing)
	clock := time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC)
	input := careplan.PickupException{StudentID: student.ID, ExceptionDate: careplan.Date(date), PickupTime: &clock, CreatedBy: staff.ID, Source: careplan.ExceptionSourceStaff}
	originalClock := clock
	id, err := records.Create(ctx, input)
	require.NoError(t, err)
	require.Positive(t, id)
	require.Equal(t, originalClock, clock, "RETURNING must not mutate the caller's clock")
	_, err = records.Create(ctx, input)
	require.Error(t, err)
	require.True(t, records.IsUniqueViolation(err), "a duplicate day must retain its conflict classification: %v", err)
	require.False(t, records.IsUniqueViolation(errors.New("database unavailable")))
	stored, err := records.FindForDate(ctx, student.ID, date)
	require.NoError(t, err)
	require.Equal(t, id, stored.ID)
	require.Equal(t, testpkg.Tenant(t), stored.TenantID)
	require.NoError(t, records.Update(ctx, *stored), "scanned TIME values must survive a round trip")
	later := clock.Add(time.Hour)
	stored.PickupTime = &later
	require.NoError(t, records.Update(ctx, *stored))
	updated, err := records.FindForDate(ctx, student.ID, date)
	require.NoError(t, err)
	require.Equal(t, "16:00", updated.PickupTime.Format("15:04"))
	require.Equal(t, staff.ID, updated.CreatedBy)
	missing, err = records.FindForDate(ctx, student.ID, date.AddDays(1))
	require.NoError(t, err)
	require.Nil(t, missing)
}

func TestNativePickupSchedulesRejectMissingDependencies(t *testing.T) {
	t.Parallel()
	_, err := compose.NewPickupSchedules(nil, nil, nil, nil, nil, nil)
	require.Error(t, err)
}
