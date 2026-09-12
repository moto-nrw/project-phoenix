package test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func CreateTestOfferingCorrection(tb testing.TB, db *bun.DB, studentID int64, source string, changedAt time.Time) int64 {
	tb.Helper()
	account := CreateTestAccount(tb, db, "correction")
	_, requestID, childID := CreateAuditAdjustmentChain(tb, db)
	row := &audit.EnrollmentOfferingAdjustment{RequestID: requestID, RequestChildID: childID, StudentID: studentID, ActorAccountID: account.ID, ActorRole: "admin", Reason: "Review fixture", Source: source, Before: []byte(`[]`), After: []byte(`[]`), ChangedAt: changedAt}
	row.SetTenantID(fixtureTenantID(tb))
	_, err := db.NewInsert().Model(row).Exec(Ctx(tb))
	require.NoError(tb, err)
	return row.ID
}
