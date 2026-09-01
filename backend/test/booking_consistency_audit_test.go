package test

import (
	"testing"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
)

func bookingConsistencyAuditTestRepository(t *testing.T) any {
	t.Helper()
	db := SetupTestDB(t)
	return auditRepo.NewBookingConsistencyRepository(auditRepo.NewRuntime(db, AuditTenantIDFromContext))
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
