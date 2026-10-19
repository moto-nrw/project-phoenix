package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var bookingAuditMonday = calendar.NewDate(2026, time.January, 5)

// The VerifyBookingConsistencyAudit* scenarios arrange their rows here and
// assert the report of the repository the Audit integration suite composes
// (database/repositories/audit/integration).

// BookingConsistencyAudit runs the Audit Platform's booking consistency audit
// for the tenant in ctx on auditDate (YYYY-MM-DD). The Audit integration
// suite adapts its repository to it, so the scenarios need not name the Audit
// Platform's report type (#2748).
type BookingConsistencyAudit func(ctx context.Context, auditDate string) (BookingConsistencyFindings, error)

// BookingConsistencyFindings carries the report fields the scenarios assert;
// TotalFindings is the report's own count.
type BookingConsistencyFindings struct {
	TenantID                        int64
	PickupProjectionMissingDays     int
	ApprovedWithoutRequiredOffering int
	ApprovedWithoutOptionalOffering int
	TotalFindings                   int
}

// VerifyBookingConsistencyAuditIgnoresRuntimeFilteredPlanningRows: raw arrival
// rows on unbooked days and broad class rosters are deliberate in booking-led
// care. This fixture includes both and pins that only actionable
// pickup/offering violations contribute to the report.
func VerifyBookingConsistencyAuditIgnoresRuntimeFilteredPlanningRows(t *testing.T, db *bun.DB, runAudit BookingConsistencyAudit) {
	ctx := Ctx(t)
	auditDate := bookingAuditMonday

	bookedStudent := CreateTestStudent(t, db, "Gebucht", "Audit", "1a")
	bookedChild := createApprovedBookingAuditChild(t, ctx, db, auditDate, bookedStudent.ID,
		enrollmentOwner.PhaseCareOfferingSelectionAtLeastOne)
	offering := CreateTestCareOffering(t, db, bookedChild.phase.ID, "Betreuung")
	offering.DaysOfWeekMode = enrollmentOwner.DaysOfWeekModeParentChoice
	offering.PickupTimes = map[string]string{"mon": "14:30"}
	updateBookingAuditModel(t, db, ctx, offering, "days_of_week_mode", "pickup_times")
	link := &auditOfferingSelection{
		RequestChildID: bookedChild.child.ID,
		CareOfferingID: offering.ID,
		SelectedDays:   []string{"mon", "tue"},
		ValidFrom:      &auditDate,
	}
	insertAuditOfferingSelection(t, db, ctx, tenant.FromContext(ctx), link)

	staff := CreateTestStaff(t, db, "Audit", "Planung")
	arrivalMonday := &StudentArrivalSchedule{
		StudentID: bookedStudent.ID, Weekday: 1, ExpectedArrival: bookingAuditWallClock(11, 45), CreatedBy: staff.ID,
	}
	insertTenantBookingAuditModel(t, db, ctx, arrivalMonday, &arrivalMonday.TenantID)
	arrivalWednesday := &StudentArrivalSchedule{
		StudentID: bookedStudent.ID, Weekday: 3, ExpectedArrival: bookingAuditWallClock(11, 45), CreatedBy: staff.ID,
	}
	insertTenantBookingAuditModel(t, db, ctx, arrivalWednesday, &arrivalWednesday.TenantID)
	offeringID := offering.ID
	pickupTuesday := &StudentPickupSchedule{
		StudentID: bookedStudent.ID, Weekday: 2, PickupTime: bookingAuditWallClock(16, 0),
		Source: SchedulePickupSourceCareOffering, CareOfferingID: &offeringID,
	}
	insertTenantBookingAuditModel(t, db, ctx, pickupTuesday, &pickupTuesday.TenantID)

	room := CreateTestRoom(t, db, "Audit")
	nonCareGroup := CreateTestActivityGroup(t, db, "Audit AG")
	tuesday := CreateTestActivityInstance(t, db, auditDate.AddDays(1), room.ID, ActivityInstanceOpts{
		ActivityGroupID: &nonCareGroup.ID,
	})
	CreateTestInstanceStudent(t, db, tuesday.ID, bookedStudent.ID, ScheduleAttendanceStatusExpected)
	thursday := CreateTestActivityInstance(t, db, auditDate.AddDays(3), room.ID, ActivityInstanceOpts{
		ActivityGroupID: &nonCareGroup.ID,
	})
	CreateTestInstanceStudent(t, db, thursday.ID, bookedStudent.ID, ScheduleAttendanceStatusExpected)
	careGroup := CreateTestActivityGroup(t, db, "Audit Betreuung")
	careGroup.Type = ActivityGroupTypeCare
	updateBookingAuditModel(t, db, ctx, careGroup, "type")
	wednesday := CreateTestActivityInstance(t, db, auditDate.AddDays(2), room.ID, ActivityInstanceOpts{
		ActivityGroupID: &careGroup.ID,
	})
	CreateTestInstanceStudent(t, db, wednesday.ID, bookedStudent.ID, ScheduleAttendanceStatusExpected)

	missingRequiredStudent := CreateTestStudent(t, db, "Pflicht", "Audit", "1b")
	createApprovedBookingAuditChild(t, ctx, db, auditDate, missingRequiredStudent.ID,
		enrollmentOwner.PhaseCareOfferingSelectionAtLeastOne)
	optionalStudent := CreateTestStudent(t, db, "Optional", "Audit", "1c")
	createApprovedBookingAuditChild(t, ctx, db, auditDate, optionalStudent.ID,
		enrollmentOwner.PhaseCareOfferingSelectionOptional)
	otherTenant := NewTenantScope(t, db)
	otherStudent := CreateTestStudentForTenant(t, db, otherTenant.TenantID, "Fremd", "Audit", "1d")
	createApprovedBookingAuditChild(t, otherTenant.Context(), db, auditDate, otherStudent.ID,
		enrollmentOwner.PhaseCareOfferingSelectionAtLeastOne)

	report, err := TenantQuery1(t, ctx, db, runAudit, auditDate.String())
	require.NoError(t, err)
	assert.Equal(t, Tenant(t), report.TenantID)
	assert.Equal(t, 1, report.PickupProjectionMissingDays)
	assert.Equal(t, 1, report.ApprovedWithoutRequiredOffering)
	assert.Equal(t, 1, report.ApprovedWithoutOptionalOffering)
	assert.Equal(t, 2, report.TotalFindings)
}

// VerifyBookingConsistencyAuditRequiresDateAndTenant pins the refusals of an
// audit without a tenant context or date.
func VerifyBookingConsistencyAuditRequiresDateAndTenant(t *testing.T, runAudit BookingConsistencyAudit) {
	_, err := runAudit(context.Background(), calendar.TodayDate().String())
	require.ErrorContains(t, err, "requires a tenant context")

	_, err = runAudit(Ctx(t), "")
	require.ErrorContains(t, err, "date is required")
}

// VerifyBookingConsistencyAuditUsesEffectiveDatesAndExceptions pins that
// arrival exceptions and links valid only later do not count as findings.
func VerifyBookingConsistencyAuditUsesEffectiveDatesAndExceptions(t *testing.T, db *bun.DB, runAudit BookingConsistencyAudit) {
	ctx := Ctx(t)
	auditDate := bookingAuditMonday
	staff := CreateTestStaff(t, db, "Audit", "Ausnahme")

	withArrivalException := CreateTestStudent(t, db, "Ankunft", "Ausnahme", "1a")
	child := createApprovedBookingAuditChild(t, ctx, db, auditDate, withArrivalException.ID,
		enrollmentOwner.PhaseCareOfferingSelectionAtLeastOne)
	offering := CreateTestCareOffering(t, db, child.phase.ID, "Montagsbetreuung")
	offering.DaysOfWeekMode = enrollmentOwner.DaysOfWeekModeParentChoice
	offering.PickupTimes = map[string]string{"mon": "14:30"}
	updateBookingAuditModel(t, db, ctx, offering, "days_of_week_mode", "pickup_times")
	arrivalLink := &auditOfferingSelection{
		RequestChildID: child.child.ID, CareOfferingID: offering.ID, SelectedDays: []string{"mon"}, ValidFrom: &auditDate,
	}
	insertAuditOfferingSelection(t, db, ctx, tenant.FromContext(ctx), arrivalLink)
	arrivalException := &StudentArrivalException{
		StudentID: withArrivalException.ID, ExceptionDate: calendar.Date(auditDate), CreatedBy: staff.ID,
	}
	insertTenantBookingAuditModel(t, db, ctx, arrivalException, &arrivalException.TenantID)

	futureStudent := CreateTestStudent(t, db, "Zukunft", "Ausnahme", "1c")
	futureChild := createApprovedBookingAuditChild(t, ctx, db, auditDate, futureStudent.ID,
		enrollmentOwner.PhaseCareOfferingSelectionAtLeastOne)
	futureOffering := CreateTestCareOffering(t, db, futureChild.phase.ID, "Spätere Betreuung")
	futureOffering.DaysOfWeekMode = enrollmentOwner.DaysOfWeekModeParentChoice
	futureOffering.PickupTimes = map[string]string{"mon": "14:30"}
	updateBookingAuditModel(t, db, ctx, futureOffering, "days_of_week_mode", "pickup_times")
	futureDate := auditDate.AddDays(1)
	futureLink := &auditOfferingSelection{
		RequestChildID: futureChild.child.ID, CareOfferingID: futureOffering.ID,
		SelectedDays: []string{"mon"}, ValidFrom: &futureDate,
	}
	insertAuditOfferingSelection(t, db, ctx, tenant.FromContext(ctx), futureLink)

	report, err := TenantQuery1(t, ctx, db, runAudit, auditDate.String())
	require.NoError(t, err)
	assert.Equal(t, 0, report.PickupProjectionMissingDays)
	assert.Equal(t, 1, report.ApprovedWithoutRequiredOffering)
}

// VerifyBookingConsistencyAuditAcceptsContinuousSplitOfferingLinks pins that
// two contiguous links of one offering cover the required offering.
func VerifyBookingConsistencyAuditAcceptsContinuousSplitOfferingLinks(t *testing.T, db *bun.DB, runAudit BookingConsistencyAudit) {
	ctx := Ctx(t)
	auditDate := bookingAuditMonday
	staff := CreateTestStaff(t, db, "Audit", "Split")
	student := CreateTestStudent(t, db, "Geteilt", "Audit", "1a")
	child := createApprovedBookingAuditChild(t, ctx, db, auditDate, student.ID,
		enrollmentOwner.PhaseCareOfferingSelectionAtLeastOne)
	offering := CreateTestCareOffering(t, db, child.phase.ID, "Geteilte Betreuung")
	offering.DaysOfWeekMode = enrollmentOwner.DaysOfWeekModeParentChoice
	offering.PickupTimes = map[string]string{"mon": "14:30"}
	updateBookingAuditModel(t, db, ctx, offering, "days_of_week_mode", "pickup_times")

	splitDate := auditDate.AddDays(15)
	phaseEndExclusive := calendar.Date(child.phase.ServiceEndDate).AddDays(1)
	firstLink := &auditOfferingSelection{
		RequestChildID: child.child.ID, CareOfferingID: offering.ID,
		SelectedDays: []string{"mon"}, ValidFrom: &auditDate, ValidUntil: &splitDate,
	}
	insertAuditOfferingSelection(t, db, ctx, tenant.FromContext(ctx), firstLink)
	secondLink := &auditOfferingSelection{
		RequestChildID: child.child.ID, CareOfferingID: offering.ID,
		SelectedDays: []string{"mon"}, ValidFrom: &splitDate, ValidUntil: &phaseEndExclusive,
	}
	insertAuditOfferingSelection(t, db, ctx, tenant.FromContext(ctx), secondLink)
	arrival := &StudentArrivalSchedule{
		StudentID: student.ID, Weekday: ScheduleWeekdayMonday,
		ExpectedArrival: bookingAuditWallClock(11, 45), CreatedBy: staff.ID,
	}
	insertTenantBookingAuditModel(t, db, ctx, arrival, &arrival.TenantID)

	report, err := TenantQuery1(t, ctx, db, runAudit, auditDate.String())
	require.NoError(t, err)
	assert.Equal(t, 0, report.ApprovedWithoutRequiredOffering)
}

type approvedBookingAuditChild struct {
	phase *enrollmentOwner.Phase
	child *enrollmentOwner.RequestChild
}

func createApprovedBookingAuditChild(
	t *testing.T,
	ctx context.Context,
	db *bun.DB,
	auditDate calendar.Date,
	studentID int64,
	selectionMode string,
) approvedBookingAuditChild {
	t.Helper()
	tenantID := tenant.FromContext(ctx)
	phase := &enrollmentOwner.Phase{
		Name:                      fmt.Sprintf("Audit-%d", studentID),
		Kind:                      enrollmentOwner.PhaseKindSchoolYear,
		ServiceStartDate:          enrollmentOwner.Date(auditDate),
		ServiceEndDate:            enrollmentOwner.Date(auditDate.AddDays(30)),
		CareOverflowMode:          enrollmentOwner.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: selectionMode,
		IsActive:                  true,
	}
	phase.TenantID = tenantID
	require.NoError(t, TenantCommand1(t, ctx, db, enrollmentOwner.New().InsertPhase, phase))
	request := &enrollmentOwner.Request{
		PhaseID:           phase.ID,
		GuardianFirstName: "Audit",
		GuardianLastName:  "Test",
		GuardianEmail:     fmt.Sprintf("booking-audit-request-%d@example.test", studentID),
		ConsentFlags:      []byte(`{}`),
		CustomData:        []byte(`{}`),
		SubmissionSource:  enrollmentOwner.RequestSourceAdminManual,
		SourceMetadata:    []byte(`{}`),
		StatusToken:       fmt.Sprintf("booking-audit-%d-%d", studentID, time.Now().UnixNano()),
		SubmittedAt:       time.Now(),
	}
	owner := enrollmentOwner.New()
	ownerCtx := WithTenantRuntime(t, ctx, db)
	require.NoError(t, TenantCommand1(t, ownerCtx, db, owner.InsertRequest, request))
	child := &enrollmentOwner.RequestChild{
		RequestID: request.ID, FirstName: "Audit", LastName: "Kind",
		DateOfBirth: "2018-01-01", CustomData: []byte(`{}`),
		Status: enrollmentOwner.ChildStatusApproved, ActivationMode: enrollmentOwner.ChildActivationImmediate,
		CreatedStudentID: &studentID,
	}
	require.NoError(t, TenantCommand1(t, ownerCtx, db, owner.InsertChild, child))
	require.Equal(t, tenantID, child.TenantID)
	return approvedBookingAuditChild{phase: phase, child: child}
}

func insertTenantBookingAuditModel(t *testing.T, db *bun.DB, ctx context.Context, model any, tenantID *int64) {
	t.Helper()
	*tenantID = tenant.FromContext(ctx)
	insertBookingAuditModel(t, db, ctx, model)
}

func insertBookingAuditModel(t *testing.T, db *bun.DB, ctx context.Context, model any) {
	t.Helper()
	_, err := db.NewInsert().Model(model).ModelTableExpr(auditFixtureTable(model)).Exec(ctx)
	require.NoError(t, err)
}

func updateBookingAuditModel(t *testing.T, db *bun.DB, ctx context.Context, model any, columns ...string) {
	t.Helper()
	_, err := db.NewUpdate().Model(model).ModelTableExpr(auditFixtureUpdateTable(model)).Column(columns...).
		WherePK().Exec(ctx)
	require.NoError(t, err)
}

func auditFixtureUpdateTable(model any) string {
	switch model.(type) {
	case *enrollmentOwner.Phase:
		return `enrollment.phases AS "phase"`
	case *CareOffering:
		return `enrollment.care_offerings AS "care_offering"`
	case *ActivityGroup:
		return `activities.groups AS "group"`
	default:
		panic(fmt.Sprintf("unsupported Audit fixture update %T", model))
	}
}

func auditFixtureTable(model any) string {
	switch model.(type) {
	case *enrollmentOwner.Phase:
		return "enrollment.phases"
	case *CareOffering:
		return "enrollment.care_offerings"
	case *auditOfferingSelection:
		return "enrollment.care_offering_bookings"
	case *StudentArrivalSchedule:
		return "schedule.student_arrival_schedules"
	case *StudentArrivalException:
		return "schedule.student_arrival_exceptions"
	case *StudentPickupSchedule:
		return "schedule.student_pickup_schedules"
	case *ActivityGroup:
		return "activities.groups"
	default:
		panic(fmt.Sprintf("unsupported Audit fixture model %T", model))
	}
}

func bookingAuditWallClock(hour, minute int) time.Time {
	return time.Date(2000, time.January, 1, hour, minute, 0, 0, time.UTC)
}
