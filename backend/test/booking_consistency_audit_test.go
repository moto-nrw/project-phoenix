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
	repo := auditRepo.NewBookingConsistencyRepository(auditRepo.NewRuntime(db, AuditTenantIDFromContext), enrollmentTest.New())
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
	err := d.db.NewSelect().TableExpr("users.students").Column("id", "status").Where("id IN (?)", bun.List(ids)).Scan(ctx, &rows)
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
