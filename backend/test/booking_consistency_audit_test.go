package test

import (
	"context"
	"testing"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	"github.com/uptrace/bun"
)

func bookingConsistencyAuditTestRepository(t *testing.T) any {
	t.Helper()
	db := SetupTestDB(t)
	repo := auditRepo.NewBookingConsistencyRepository(auditRepo.NewRuntime(db, AuditTenantIDFromContext))
	// The audit reads the alumnus exclusion through the People Directory port
	// (#2662); this package serves it straight from the table it may read.
	repo.(interface {
		BindStudentDirectory(auditRepo.StudentDirectory)
	}).BindStudentDirectory(rawStudentDirectory{db: db})
	return repo
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
