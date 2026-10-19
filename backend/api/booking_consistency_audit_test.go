package api

import (
	"context"
	"testing"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The booking consistency audit over the owners the root binds it to:
// Enrollment's approved bookings, Care Plan's offerings and People
// Directory's alumni (#4072). The Audit Platform names none of them.

func bookingConsistencyAuditUnderTest(t *testing.T, db *bun.DB) testpkg.BookingConsistencyAudit {
	t.Helper()
	carePlan, err := owners.NewUnobservedCarePlan(testpkg.NewDatabase(t, db), owners.MustNewUnobservedTimetable(testpkg.NewDatabase(t, db)), owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, db)))
	require.NoError(t, err)
	audit, err := owners.NewBookingConsistencyAudit(testpkg.NewDatabase(t, db), owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, db)), carePlan)
	require.NoError(t, err)
	return func(ctx context.Context, auditDate string) (testpkg.BookingConsistencyFindings, error) {
		report, err := audit(ctx, auditModels.Date(auditDate))
		if err != nil {
			return testpkg.BookingConsistencyFindings{}, err
		}
		return testpkg.BookingConsistencyFindings{
			TenantID:                        report.TenantID,
			PickupProjectionMissingDays:     report.PickupProjectionMissingDays,
			ApprovedWithoutRequiredOffering: report.ApprovedWithoutRequiredOffering,
			ApprovedWithoutOptionalOffering: report.ApprovedWithoutOptionalOffering,
			TotalFindings:                   report.TotalFindings(),
		}, nil
	}
}

func TestBookingConsistencyAuditIgnoresRuntimeFilteredPlanningRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.VerifyBookingConsistencyAuditIgnoresRuntimeFilteredPlanningRows(t, db, bookingConsistencyAuditUnderTest(t, db))
}

func TestBookingConsistencyAuditRequiresDateAndTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.VerifyBookingConsistencyAuditRequiresDateAndTenant(t, bookingConsistencyAuditUnderTest(t, db))
}

func TestBookingConsistencyAuditUsesEffectiveDatesAndExceptions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.VerifyBookingConsistencyAuditUsesEffectiveDatesAndExceptions(t, db, bookingConsistencyAuditUnderTest(t, db))
}

func TestBookingConsistencyAuditAcceptsContinuousSplitOfferingLinks(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.VerifyBookingConsistencyAuditAcceptsContinuousSplitOfferingLinks(t, db, bookingConsistencyAuditUnderTest(t, db))
}
