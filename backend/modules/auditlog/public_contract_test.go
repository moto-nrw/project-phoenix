package auditlog_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	"github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestPublicAuditContractReturnsFrozenCorrectionAndCursor(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Audit", "Contract", "1A")
	at := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	first := testpkg.CreateTestOfferingCorrection(t, db, student.ID, "direct", at)
	second := testpkg.CreateTestOfferingCorrection(t, db, student.ID, "direct", at)
	testpkg.CreateTestOfferingCorrection(t, db, student.ID, "request", at.Add(time.Hour))
	query, err := compose.NewCorrectionQueries(db, func(auditlog.Observation) {})
	require.NoError(t, err)
	rows, err := query.ListDirectCorrections(testpkg.Ctx(t), auditlog.CorrectionFilter{Limit: 1})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, second, rows[0].ID)
	require.Equal(t, student.ID, rows[0].StudentID)
	require.JSONEq(t, "[]", string(rows[0].Before))
	require.JSONEq(t, "[]", string(rows[0].After))
	rows, err = query.ListDirectCorrections(testpkg.Ctx(t), auditlog.CorrectionFilter{Limit: 1, BeforeInstant: at, BeforeID: second})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, first, rows[0].ID)
}
