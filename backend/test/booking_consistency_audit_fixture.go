package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
	enrollmentModel "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var bookingAuditMonday = timezone.NewDate(2026, time.January, 5)

// Raw arrival rows on unbooked days and broad class rosters are deliberate in
// booking-led care. This fixture includes both and pins that only actionable
// pickup/offering violations contribute to the report.
func verifyBookingConsistencyAuditIgnoresRuntimeFilteredPlanningRows(t *testing.T, db *bun.DB, repository any) {
	ctx := Ctx(t)
	repo := repository.(auditModel.BookingConsistencyRepository)
	auditDate := bookingAuditMonday

	bookedStudent := CreateTestStudent(t, db, "Gebucht", "Audit", "1a")
	bookedChild := createApprovedBookingAuditChild(t, ctx, db, auditDate, bookedStudent.ID,
		enrollmentModel.PhaseCareOfferingSelectionAtLeastOne)
	offering := CreateTestCareOffering(t, db, bookedChild.phase.ID, "Betreuung")
	offering.DaysOfWeekMode = enrollmentModel.DaysOfWeekModeParentChoice
	offering.PickupTimes = map[string]string{"mon": "14:30"}
	updateBookingAuditModel(t, db, ctx, offering, "days_of_week_mode", "pickup_times")
	link := &enrollmentModel.RequestChildOffering{
		RequestChildID: bookedChild.child.ID,
		CareOfferingID: offering.ID,
		SelectedDays:   []string{"mon", "tue"},
		ValidFrom:      &auditDate,
	}
	insertTenantBookingAuditModel(t, db, ctx, link)

	staff := CreateTestStaff(t, db, "Audit", "Planung")
	arrivalMonday := &scheduleModel.StudentArrivalSchedule{
		StudentID: bookedStudent.ID, Weekday: 1, ExpectedArrival: bookingAuditWallClock(11, 45), CreatedBy: staff.ID,
	}
	insertTenantBookingAuditModel(t, db, ctx, arrivalMonday)
	arrivalWednesday := &scheduleModel.StudentArrivalSchedule{
		StudentID: bookedStudent.ID, Weekday: 3, ExpectedArrival: bookingAuditWallClock(11, 45), CreatedBy: staff.ID,
	}
	insertTenantBookingAuditModel(t, db, ctx, arrivalWednesday)
	offeringID := offering.ID
	pickupTuesday := &scheduleModel.StudentPickupSchedule{
		StudentID: bookedStudent.ID, Weekday: 2, PickupTime: bookingAuditWallClock(16, 0),
		Source: scheduleModel.PickupScheduleSourceCareOffering, CareOfferingID: &offeringID,
	}
	insertTenantBookingAuditModel(t, db, ctx, pickupTuesday)

	room := CreateTestRoom(t, db, "Audit")
	nonCareGroup := CreateTestActivityGroup(t, db, "Audit AG")
	tuesday := CreateTestActivityInstance(t, db, auditDate.AddDays(1), room.ID, ActivityInstanceOpts{
		ActivityGroupID: &nonCareGroup.ID,
	})
	CreateTestInstanceStudent(t, db, tuesday.ID, bookedStudent.ID, scheduleModel.AttendanceStatusExpected)
	thursday := CreateTestActivityInstance(t, db, auditDate.AddDays(3), room.ID, ActivityInstanceOpts{
		ActivityGroupID: &nonCareGroup.ID,
	})
	CreateTestInstanceStudent(t, db, thursday.ID, bookedStudent.ID, scheduleModel.AttendanceStatusExpected)
	careGroup := CreateTestActivityGroup(t, db, "Audit Betreuung")
	careGroup.Type = activitiesModel.GroupTypeCare
	updateBookingAuditModel(t, db, ctx, careGroup, "type")
	wednesday := CreateTestActivityInstance(t, db, auditDate.AddDays(2), room.ID, ActivityInstanceOpts{
		ActivityGroupID: &careGroup.ID,
	})
	CreateTestInstanceStudent(t, db, wednesday.ID, bookedStudent.ID, scheduleModel.AttendanceStatusExpected)

	missingRequiredStudent := CreateTestStudent(t, db, "Pflicht", "Audit", "1b")
	createApprovedBookingAuditChild(t, ctx, db, auditDate, missingRequiredStudent.ID,
		enrollmentModel.PhaseCareOfferingSelectionAtLeastOne)
	optionalStudent := CreateTestStudent(t, db, "Optional", "Audit", "1c")
	createApprovedBookingAuditChild(t, ctx, db, auditDate, optionalStudent.ID,
		enrollmentModel.PhaseCareOfferingSelectionOptional)
	otherTenant := NewTenantScope(t, db)
	otherStudent := CreateTestStudentForTenant(t, db, otherTenant.TenantID, "Fremd", "Audit", "1d")
	createApprovedBookingAuditChild(t, otherTenant.Context(), db, auditDate, otherStudent.ID,
		enrollmentModel.PhaseCareOfferingSelectionAtLeastOne)

	report, err := repo.Audit(ctx, auditModel.Date(auditDate.String()))
	require.NoError(t, err)
	assert.Equal(t, Tenant(t), report.TenantID)
	assert.Equal(t, 1, report.PickupProjectionMissingDays)
	assert.Equal(t, 1, report.ApprovedWithoutRequiredOffering)
	assert.Equal(t, 1, report.ApprovedWithoutOptionalOffering)
	assert.Equal(t, 2, report.TotalFindings())
}

func verifyBookingConsistencyAuditRequiresDateAndTenant(t *testing.T, repository any) {
	repo := repository.(auditModel.BookingConsistencyRepository)
	_, err := repo.Audit(context.Background(), auditModel.Date(timezone.TodayDate().String()))
	require.ErrorContains(t, err, "requires a tenant context")

	_, err = repo.Audit(Ctx(t), auditModel.Date(""))
	require.ErrorContains(t, err, "date is required")
}

func verifyBookingConsistencyAuditUsesEffectiveDatesAndExceptions(t *testing.T, db *bun.DB, repository any) {
	ctx := Ctx(t)
	repo := repository.(auditModel.BookingConsistencyRepository)
	auditDate := bookingAuditMonday
	staff := CreateTestStaff(t, db, "Audit", "Ausnahme")

	withArrivalException := CreateTestStudent(t, db, "Ankunft", "Ausnahme", "1a")
	child := createApprovedBookingAuditChild(t, ctx, db, auditDate, withArrivalException.ID,
		enrollmentModel.PhaseCareOfferingSelectionAtLeastOne)
	offering := CreateTestCareOffering(t, db, child.phase.ID, "Montagsbetreuung")
	offering.DaysOfWeekMode = enrollmentModel.DaysOfWeekModeParentChoice
	offering.PickupTimes = map[string]string{"mon": "14:30"}
	updateBookingAuditModel(t, db, ctx, offering, "days_of_week_mode", "pickup_times")
	arrivalLink := &enrollmentModel.RequestChildOffering{
		RequestChildID: child.child.ID, CareOfferingID: offering.ID, SelectedDays: []string{"mon"}, ValidFrom: &auditDate,
	}
	insertTenantBookingAuditModel(t, db, ctx, arrivalLink)
	arrivalException := &scheduleModel.StudentArrivalException{
		StudentID: withArrivalException.ID, ExceptionDate: scheduleModel.Date(auditDate), CreatedBy: staff.ID,
	}
	insertTenantBookingAuditModel(t, db, ctx, arrivalException)

	futureStudent := CreateTestStudent(t, db, "Zukunft", "Ausnahme", "1c")
	futureChild := createApprovedBookingAuditChild(t, ctx, db, auditDate, futureStudent.ID,
		enrollmentModel.PhaseCareOfferingSelectionAtLeastOne)
	futureOffering := CreateTestCareOffering(t, db, futureChild.phase.ID, "Spätere Betreuung")
	futureOffering.DaysOfWeekMode = enrollmentModel.DaysOfWeekModeParentChoice
	futureOffering.PickupTimes = map[string]string{"mon": "14:30"}
	updateBookingAuditModel(t, db, ctx, futureOffering, "days_of_week_mode", "pickup_times")
	futureDate := auditDate.AddDays(1)
	futureLink := &enrollmentModel.RequestChildOffering{
		RequestChildID: futureChild.child.ID, CareOfferingID: futureOffering.ID,
		SelectedDays: []string{"mon"}, ValidFrom: &futureDate,
	}
	insertTenantBookingAuditModel(t, db, ctx, futureLink)

	report, err := repo.Audit(ctx, auditModel.Date(auditDate.String()))
	require.NoError(t, err)
	assert.Equal(t, 0, report.PickupProjectionMissingDays)
	assert.Equal(t, 1, report.ApprovedWithoutRequiredOffering)
}

func verifyBookingConsistencyAuditAcceptsContinuousSplitOfferingLinks(t *testing.T, db *bun.DB, repository any) {
	ctx := Ctx(t)
	repo := repository.(auditModel.BookingConsistencyRepository)
	auditDate := bookingAuditMonday
	staff := CreateTestStaff(t, db, "Audit", "Split")
	student := CreateTestStudent(t, db, "Geteilt", "Audit", "1a")
	child := createApprovedBookingAuditChild(t, ctx, db, auditDate, student.ID,
		enrollmentModel.PhaseCareOfferingSelectionAtLeastOne)
	offering := CreateTestCareOffering(t, db, child.phase.ID, "Geteilte Betreuung")
	offering.DaysOfWeekMode = enrollmentModel.DaysOfWeekModeParentChoice
	offering.PickupTimes = map[string]string{"mon": "14:30"}
	updateBookingAuditModel(t, db, ctx, offering, "days_of_week_mode", "pickup_times")

	splitDate := auditDate.AddDays(15)
	phaseEndExclusive := child.phase.ServiceEndDate.AddDays(1)
	firstLink := &enrollmentModel.RequestChildOffering{
		RequestChildID: child.child.ID, CareOfferingID: offering.ID,
		SelectedDays: []string{"mon"}, ValidFrom: &auditDate, ValidUntil: &splitDate,
	}
	insertTenantBookingAuditModel(t, db, ctx, firstLink)
	secondLink := &enrollmentModel.RequestChildOffering{
		RequestChildID: child.child.ID, CareOfferingID: offering.ID,
		SelectedDays: []string{"mon"}, ValidFrom: &splitDate, ValidUntil: &phaseEndExclusive,
	}
	insertTenantBookingAuditModel(t, db, ctx, secondLink)
	arrival := &scheduleModel.StudentArrivalSchedule{
		StudentID: student.ID, Weekday: scheduleModel.WeekdayMonday,
		ExpectedArrival: bookingAuditWallClock(11, 45), CreatedBy: staff.ID,
	}
	insertTenantBookingAuditModel(t, db, ctx, arrival)

	report, err := repo.Audit(ctx, auditModel.Date(auditDate.String()))
	require.NoError(t, err)
	assert.Equal(t, 0, report.ApprovedWithoutRequiredOffering)
}

type approvedBookingAuditChild struct {
	phase *enrollmentModel.Phase
	child *enrollmentModel.RequestChild
}

func createApprovedBookingAuditChild(
	t *testing.T,
	ctx context.Context,
	db *bun.DB,
	auditDate timezone.Date,
	studentID int64,
	selectionMode string,
) approvedBookingAuditChild {
	t.Helper()
	tenantID := auditModel.TenantIDFromContext(ctx)
	phase := &enrollmentModel.Phase{
		Name:                      fmt.Sprintf("Audit-%d", studentID),
		Kind:                      enrollmentModel.PhaseKindSchoolYear,
		ServiceStartDate:          auditDate,
		ServiceEndDate:            auditDate.AddDays(30),
		CareOverflowMode:          enrollmentModel.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: selectionMode,
		IsActive:                  true,
	}
	insertTenantBookingAuditModel(t, db, ctx, phase)
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
	insertTenantBookingAuditModel(t, db, ctx, request)
	child := &enrollmentModel.RequestChild{
		RequestID: request.ID, FirstName: "Audit", LastName: "Kind",
		DateOfBirth: timezone.NewDate(2018, time.January, 1), CustomData: map[string]any{},
		Status: enrollmentModel.ChildStatusApproved, ActivationMode: enrollmentModel.ChildActivationImmediate,
		CreatedStudentID: &studentID,
	}
	insertTenantBookingAuditModel(t, db, ctx, child)
	require.Equal(t, tenantID, child.TenantID)
	return approvedBookingAuditChild{phase: phase, child: child}
}

func AuditTenantIDFromContext(ctx context.Context) int64 {
	return auditModel.TenantIDFromContext(ctx)
}

func insertTenantBookingAuditModel(t *testing.T, db *bun.DB, ctx context.Context, model interface{ SetTenantID(int64) }) {
	t.Helper()
	model.SetTenantID(auditModel.TenantIDFromContext(ctx))
	insertBookingAuditModel(t, db, ctx, model)
}

func insertBookingAuditModel(t *testing.T, db *bun.DB, ctx context.Context, model any) {
	t.Helper()
	_, err := db.NewInsert().Model(model).ModelTableExpr(auditFixtureTable(model)).Exec(ctx)
	require.NoError(t, err)
}

func updateBookingAuditModel(t *testing.T, db *bun.DB, ctx context.Context, model any, columns ...string) {
	t.Helper()
	_, err := db.NewUpdate().Model(model).ModelTableExpr(auditFixtureUpdateTable(model)).Column(columns...).WherePK().Exec(ctx)
	require.NoError(t, err)
}

func auditFixtureUpdateTable(model any) string {
	switch model.(type) {
	case *enrollmentModel.Phase:
		return `enrollment.phases AS "phase"`
	case *enrollmentModel.CareOffering:
		return `enrollment.care_offerings AS "care_offering"`
	case *activitiesModel.Group:
		return `activities.groups AS "group"`
	default:
		panic(fmt.Sprintf("unsupported Audit fixture update %T", model))
	}
}

func auditFixtureTable(model any) string {
	switch model.(type) {
	case *enrollmentModel.Phase:
		return "enrollment.phases"
	case *enrollmentModel.Request:
		return "enrollment.requests"
	case *enrollmentModel.RequestChild:
		return "enrollment.request_children"
	case *enrollmentModel.CareOffering:
		return "enrollment.care_offerings"
	case *enrollmentModel.RequestChildOffering:
		return "enrollment.request_child_offerings"
	case *scheduleModel.StudentArrivalSchedule:
		return "schedule.student_arrival_schedules"
	case *scheduleModel.StudentArrivalException:
		return "schedule.student_arrival_exceptions"
	case *scheduleModel.StudentPickupSchedule:
		return "schedule.student_pickup_schedules"
	case *activitiesModel.Group:
		return "activities.groups"
	default:
		panic(fmt.Sprintf("unsupported Audit fixture model %T", model))
	}
}

func bookingAuditWallClock(hour, minute int) time.Time {
	return time.Date(2000, time.January, 1, hour, minute, 0, 0, time.UTC)
}
