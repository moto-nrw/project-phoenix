package audit_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModel "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBookingConsistencyAuditFindsProjectionDrift(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db)
	auditDate := nextMonday(timezone.TodayDate())

	bookedStudent := testpkg.CreateTestStudent(t, db, "Gebucht", "Audit", "1a")
	bookedChild := createApprovedChildForStudent(t, ctx, repos, auditDate, bookedStudent.ID,
		enrollmentModel.PhaseCareOfferingSelectionAtLeastOne)
	offering := testpkg.CreateTestCareOffering(t, db, bookedChild.phase.ID, "Betreuung")
	offering.DaysOfWeekMode = enrollmentModel.DaysOfWeekModeParentChoice
	offering.PickupTimes = map[string]string{"mon": "14:30"}
	require.NoError(t, repos.CareOffering.Update(ctx, offering))
	require.NoError(t, repos.RequestChildOffering.Create(ctx, &enrollmentModel.RequestChildOffering{
		RequestChildID: bookedChild.child.ID,
		CareOfferingID: offering.ID,
		SelectedDays:   []string{"mon", "tue"},
		ValidFrom:      &auditDate,
	}))

	staff := testpkg.CreateTestStaff(t, db, "Audit", "Planung")
	require.NoError(t, repos.StudentArrivalSchedule.Create(ctx, &scheduleModel.StudentArrivalSchedule{
		StudentID: bookedStudent.ID, Weekday: 1, ExpectedArrival: wallClock(11, 45), CreatedBy: staff.ID,
	}))
	require.NoError(t, repos.StudentArrivalSchedule.Create(ctx, &scheduleModel.StudentArrivalSchedule{
		StudentID: bookedStudent.ID, Weekday: 3, ExpectedArrival: wallClock(11, 45), CreatedBy: staff.ID,
	}))
	offeringID := offering.ID
	require.NoError(t, repos.StudentPickupSchedule.Create(ctx, &scheduleModel.StudentPickupSchedule{
		StudentID: bookedStudent.ID, Weekday: 2, PickupTime: wallClock(16, 0),
		Source: scheduleModel.PickupScheduleSourceCareOffering, CareOfferingID: &offeringID,
	}))

	room := testpkg.CreateTestRoom(t, db, "Audit")
	nonCareGroup := testpkg.CreateTestActivityGroup(t, db, "Audit AG")
	tuesday := testpkg.CreateTestActivityInstance(t, db, auditDate.AddDays(1), room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &nonCareGroup.ID,
	})
	testpkg.CreateTestInstanceStudent(t, db, tuesday.ID, bookedStudent.ID, scheduleModel.AttendanceStatusExpected)
	thursday := testpkg.CreateTestActivityInstance(t, db, auditDate.AddDays(3), room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &nonCareGroup.ID,
	})
	testpkg.CreateTestInstanceStudent(t, db, thursday.ID, bookedStudent.ID, scheduleModel.AttendanceStatusExpected)
	careGroup := testpkg.CreateTestActivityGroup(t, db, "Audit Betreuung")
	careGroup.Type = activitiesModel.GroupTypeCare
	require.NoError(t, repos.ActivityGroup.Update(ctx, careGroup))
	wednesday := testpkg.CreateTestActivityInstance(t, db, auditDate.AddDays(2), room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &careGroup.ID,
	})
	testpkg.CreateTestInstanceStudent(t, db, wednesday.ID, bookedStudent.ID, scheduleModel.AttendanceStatusExpected)

	missingRequiredStudent := testpkg.CreateTestStudent(t, db, "Pflicht", "Audit", "1b")
	createApprovedChildForStudent(t, ctx, repos, auditDate, missingRequiredStudent.ID,
		enrollmentModel.PhaseCareOfferingSelectionAtLeastOne)
	optionalStudent := testpkg.CreateTestStudent(t, db, "Optional", "Audit", "1c")
	createApprovedChildForStudent(t, ctx, repos, auditDate, optionalStudent.ID,
		enrollmentModel.PhaseCareOfferingSelectionOptional)
	otherTenant := testpkg.NewTenantScope(t, db)
	otherStudent := testpkg.CreateTestStudentForTenant(t, db, otherTenant.TenantID, "Fremd", "Audit", "1d")
	createApprovedChildForStudent(t, otherTenant.Context(), repos, auditDate, otherStudent.ID,
		enrollmentModel.PhaseCareOfferingSelectionAtLeastOne)

	report, err := auditRepo.NewBookingConsistencyRepository(db).Audit(ctx, auditDate)
	require.NoError(t, err)
	assert.Equal(t, testpkg.Tenant(t), report.TenantID)
	assert.Equal(t, 1, report.PickupProjectionMissingDays)
	assert.Equal(t, 1, report.ArrivalWithoutBookingDays)
	assert.Equal(t, 1, report.BookingWithoutArrivalDays)
	assert.Equal(t, 1, report.PlannedWithoutBookingRows)
	assert.Equal(t, 1, report.ApprovedWithoutRequiredOffering)
	assert.Equal(t, 1, report.ApprovedWithoutOptionalOffering)
	assert.Equal(t, 5, report.TotalFindings())
}

func TestBookingConsistencyAuditRequiresDateAndTenant(t *testing.T) {
	t.Parallel()

	repo := auditRepo.NewBookingConsistencyRepository(nil)
	_, err := repo.Audit(context.Background(), timezone.TodayDate())
	require.ErrorContains(t, err, "requires a tenant context")

	_, err = repo.Audit(testpkg.Ctx(t), timezone.Date{})
	require.ErrorContains(t, err, "date is required")
}

func TestBookingConsistencyAuditUsesEffectiveDatesAndExceptions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db)
	auditDate := nextMonday(timezone.TodayDate())
	staff := testpkg.CreateTestStaff(t, db, "Audit", "Ausnahme")

	withArrivalException := testpkg.CreateTestStudent(t, db, "Ankunft", "Ausnahme", "1a")
	child := createApprovedChildForStudent(t, ctx, repos, auditDate, withArrivalException.ID,
		enrollmentModel.PhaseCareOfferingSelectionAtLeastOne)
	offering := testpkg.CreateTestCareOffering(t, db, child.phase.ID, "Montagsbetreuung")
	offering.DaysOfWeekMode = enrollmentModel.DaysOfWeekModeParentChoice
	offering.PickupTimes = map[string]string{"mon": "14:30"}
	require.NoError(t, repos.CareOffering.Update(ctx, offering))
	require.NoError(t, repos.RequestChildOffering.Create(ctx, &enrollmentModel.RequestChildOffering{
		RequestChildID: child.child.ID, CareOfferingID: offering.ID, SelectedDays: []string{"mon"}, ValidFrom: &auditDate,
	}))
	require.NoError(t, repos.StudentArrivalException.Create(ctx, &scheduleModel.StudentArrivalException{
		StudentID: withArrivalException.ID, ExceptionDate: auditDate, CreatedBy: staff.ID,
	}))

	futureStudent := testpkg.CreateTestStudent(t, db, "Zukunft", "Ausnahme", "1c")
	futureChild := createApprovedChildForStudent(t, ctx, repos, auditDate, futureStudent.ID,
		enrollmentModel.PhaseCareOfferingSelectionAtLeastOne)
	futureOffering := testpkg.CreateTestCareOffering(t, db, futureChild.phase.ID, "Spätere Betreuung")
	futureOffering.DaysOfWeekMode = enrollmentModel.DaysOfWeekModeParentChoice
	futureOffering.PickupTimes = map[string]string{"mon": "14:30"}
	require.NoError(t, repos.CareOffering.Update(ctx, futureOffering))
	futureDate := auditDate.AddDays(1)
	require.NoError(t, repos.RequestChildOffering.Create(ctx, &enrollmentModel.RequestChildOffering{
		RequestChildID: futureChild.child.ID, CareOfferingID: futureOffering.ID,
		SelectedDays: []string{"mon"}, ValidFrom: &futureDate,
	}))

	report, err := auditRepo.NewBookingConsistencyRepository(db).Audit(ctx, auditDate)
	require.NoError(t, err)
	assert.Equal(t, 0, report.PickupProjectionMissingDays)
	assert.Equal(t, 0, report.BookingWithoutArrivalDays)
	assert.Equal(t, 1, report.ApprovedWithoutRequiredOffering)
}

func TestBookingConsistencyAuditAcceptsContinuousSplitOfferingLinks(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db)
	auditDate := nextMonday(timezone.TodayDate())
	staff := testpkg.CreateTestStaff(t, db, "Audit", "Split")
	student := testpkg.CreateTestStudent(t, db, "Geteilt", "Audit", "1a")
	child := createApprovedChildForStudent(t, ctx, repos, auditDate, student.ID,
		enrollmentModel.PhaseCareOfferingSelectionAtLeastOne)
	offering := testpkg.CreateTestCareOffering(t, db, child.phase.ID, "Geteilte Betreuung")
	offering.DaysOfWeekMode = enrollmentModel.DaysOfWeekModeParentChoice
	offering.PickupTimes = map[string]string{"mon": "14:30"}
	require.NoError(t, repos.CareOffering.Update(ctx, offering))

	splitDate := auditDate.AddDays(15)
	phaseEndExclusive := child.phase.ServiceEndDate.AddDays(1)
	require.NoError(t, repos.RequestChildOffering.Create(ctx, &enrollmentModel.RequestChildOffering{
		RequestChildID: child.child.ID, CareOfferingID: offering.ID,
		SelectedDays: []string{"mon"}, ValidFrom: &auditDate, ValidUntil: &splitDate,
	}))
	require.NoError(t, repos.RequestChildOffering.Create(ctx, &enrollmentModel.RequestChildOffering{
		RequestChildID: child.child.ID, CareOfferingID: offering.ID,
		SelectedDays: []string{"mon"}, ValidFrom: &splitDate, ValidUntil: &phaseEndExclusive,
	}))
	require.NoError(t, repos.StudentArrivalSchedule.Create(ctx, &scheduleModel.StudentArrivalSchedule{
		StudentID: student.ID, Weekday: scheduleModel.WeekdayMonday, ExpectedArrival: wallClock(11, 45), CreatedBy: staff.ID,
	}))

	report, err := auditRepo.NewBookingConsistencyRepository(db).Audit(ctx, auditDate)
	require.NoError(t, err)
	assert.Equal(t, 0, report.ApprovedWithoutRequiredOffering)
	assert.Equal(t, 0, report.BookingWithoutArrivalDays)
}

type approvedAuditChild struct {
	phase *enrollmentModel.Phase
	child *enrollmentModel.RequestChild
}

func createApprovedChildForStudent(
	t *testing.T,
	ctx context.Context,
	repos *repositories.Factory,
	auditDate timezone.Date,
	studentID int64,
	selectionMode string,
) approvedAuditChild {
	t.Helper()
	phase := &enrollmentModel.Phase{
		Name:                      fmt.Sprintf("Audit-%d", studentID),
		Kind:                      enrollmentModel.PhaseKindSchoolYear,
		ServiceStartDate:          auditDate,
		ServiceEndDate:            auditDate.AddDays(30),
		CareOverflowMode:          enrollmentModel.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: selectionMode,
		IsActive:                  true,
	}
	require.NoError(t, repos.Phase.Create(ctx, phase))
	request := &enrollmentModel.Request{
		PhaseID:           phase.ID,
		GuardianFirstName: "Audit",
		GuardianLastName:  "Test",
		GuardianEmail:     fmt.Sprintf("booking-audit-request-%d@example.test", studentID),
		ConsentFlags:      map[string]any{},
		CustomData:        map[string]any{},
		SubmissionSource:  enrollmentModel.RequestSourceAdminManual,
		SourceMetadata:    map[string]any{},
		StatusToken:       fmt.Sprintf("booking-audit-%d-%d", studentID, time.Now().UnixNano()),
		SubmittedAt:       time.Now(),
	}
	require.NoError(t, repos.Request.Create(ctx, request))
	child := &enrollmentModel.RequestChild{
		RequestID: request.ID, FirstName: "Audit", LastName: "Kind",
		DateOfBirth: timezone.NewDate(2018, time.January, 1), CustomData: map[string]any{},
		Status: enrollmentModel.ChildStatusApproved, ActivationMode: enrollmentModel.ChildActivationImmediate,
		CreatedStudentID: &studentID,
	}
	require.NoError(t, repos.RequestChild.Create(ctx, child))
	return approvedAuditChild{phase: phase, child: child}
}

func nextMonday(date timezone.Date) timezone.Date {
	delta := (int(time.Monday) - int(date.Weekday()) + 7) % 7
	return date.AddDays(delta)
}

func wallClock(hour, minute int) time.Time {
	return time.Date(2000, time.January, 1, hour, minute, 0, 0, time.UTC)
}
