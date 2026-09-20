package test

import (
	"context"
	"testing"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanTest "github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	"github.com/uptrace/bun"
)

func bookingConsistencyAuditTestRepository(t *testing.T) any {
	t.Helper()
	db := SetupTestDB(t)
	repo := auditRepo.NewBookingConsistencyRepository(auditRepo.NewRuntime(db, AuditTenantIDFromContext), bookingAuditEnrollment{Module: enrollmentTest.New(), bookings: carePlanTest.NewOfferingBookings()})
	// The audit reads the alumnus exclusion through the People Directory port
	// (#2662); this package serves it straight from the table it may read.
	repo.(interface {
		BindStudentDirectory(auditRepo.StudentDirectory)
	}).BindStudentDirectory(rawStudentDirectory{db: db})
	repo.(interface {
		BindCarePlan(auditRepo.CareOfferingDirectory)
	}).BindCarePlan(testCareOfferingDirectory{query: carePlanTest.NewCarePlan(t, db)})
	return repo
}

type bookingAuditEnrollment struct {
	*enrollmentTest.Module
	bookings careplan.OfferingBookingQueries
}

func (a bookingAuditEnrollment) ApprovedBookingOfferingLinks(ctx context.Context) ([]enrollmentTest.CareOfferingLink, error) {
	children, err := a.CareExitApplicationLinks(ctx, nil)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		if child.Status == enrollmentTest.ChildStatusApproved {
			ids = append(ids, child.ID)
		}
	}
	bookings, err := a.bookings.CareOfferingBookingHistory(ctx, ids)
	if err != nil {
		return nil, err
	}
	rows := make([]enrollmentTest.CareOfferingLink, 0, len(bookings))
	for _, booking := range bookings {
		row := enrollmentTest.CareOfferingLink{ID: booking.ID, TenantID: booking.TenantID, RequestChildID: booking.RequestChildID, CareOfferingID: booking.CareOfferingID, SelectedDays: booking.EffectiveSelectedDays()}
		if booking.ValidFrom != nil {
			row.ValidFrom = new(enrollmentTest.Date(*booking.ValidFrom))
		}
		if booking.ValidUntil != nil {
			row.ValidUntil = new(enrollmentTest.Date(*booking.ValidUntil))
		}
		rows = append(rows, row)
	}
	return rows, nil
}

type testCareOfferingDirectory struct{ query careplan.Query }

func (d testCareOfferingDirectory) ListCareOfferings(ctx context.Context) ([]auditRepo.CareOfferingProjection, error) {
	values, err := d.query.ListCareOfferings(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderID})
	if err != nil {
		return nil, err
	}
	result := make([]auditRepo.CareOfferingProjection, 0, len(values))
	for _, value := range values {
		result = append(result, auditRepo.CareOfferingProjection{
			ID: value.ID, TenantID: value.TenantID, PhaseID: value.PhaseID,
			DaysOfWeekMode: value.DaysOfWeekMode, AvailableDays: value.AvailableDays,
			IsActive: value.IsActive, IsRequired: value.IsRequired,
			CountsAsCare: value.CountsAsCare, PickupTimes: value.PickupTimes,
		})
	}
	return result, nil
}

type rawStudentDirectory struct{ db *bun.DB }

func (d rawStudentDirectory) ListStudentsByID(ctx context.Context, ids []int64) ([]auditRepo.DirectoryStudent, error) {
	var rows []struct {
		ID     int64  `bun:"id"`
		Status string `bun:"status"`
	}
	err := d.db.NewSelect().TableExpr("users.student_school_memberships").ColumnExpr("student_profile_id AS id, status").Where("student_profile_id IN (?) AND deleted_at IS NULL", bun.List(ids)).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	result := make([]auditRepo.DirectoryStudent, 0, len(rows))
	for _, row := range rows {
		result = append(result, auditRepo.DirectoryStudent{ID: row.ID, Alumnus: row.Status == "alumnus"})
	}
	return result, nil
}

func TestBookingConsistencyAuditIgnoresRuntimeFilteredPlanningRows(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	verifyBookingConsistencyAuditIgnoresRuntimeFilteredPlanningRows(t, db, bookingConsistencyAuditTestRepository(t))
}

func TestBookingConsistencyAuditRequiresDateAndTenant(t *testing.T) {
	t.Parallel()

	verifyBookingConsistencyAuditRequiresDateAndTenant(t, bookingConsistencyAuditTestRepository(t))
}

func TestBookingConsistencyAuditUsesEffectiveDatesAndExceptions(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	verifyBookingConsistencyAuditUsesEffectiveDatesAndExceptions(t, db, bookingConsistencyAuditTestRepository(t))
}

func TestBookingConsistencyAuditAcceptsContinuousSplitOfferingLinks(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	verifyBookingConsistencyAuditAcceptsContinuousSplitOfferingLinks(t, db, bookingConsistencyAuditTestRepository(t))
}
