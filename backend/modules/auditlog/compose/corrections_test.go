package compose

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}

func TestCorrectionQueriesFilterSourcePageAndTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Audit", "Review", "1a")
	instant := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	older := testpkg.CreateTestOfferingCorrection(t, db, student.ID, "direct", instant)
	newer := testpkg.CreateTestOfferingCorrection(t, db, student.ID, "direct", instant)
	testpkg.CreateTestOfferingCorrection(t, db, student.ID, "request", instant.Add(time.Hour))
	testpkg.CreateTestOfferingCorrection(t, db, student.ID, "unknown", instant.Add(time.Hour))
	var observations []auditlog.Observation
	query, err := NewCorrectionQueries(db, func(value auditlog.Observation) { observations = append(observations, value) })
	require.NoError(t, err)
	rows, err := query.ListDirectCorrections(testpkg.Ctx(t), auditlog.CorrectionFilter{Limit: 1})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, newer, rows[0].ID)
	require.Equal(t, student.ID, rows[0].StudentID)
	require.JSONEq(t, `[]`, string(rows[0].Before))
	require.EqualValues(t, 1, observations[0].Queries)
	rows, err = query.ListDirectCorrections(testpkg.Ctx(t), auditlog.CorrectionFilter{Limit: 1, BeforeInstant: instant, BeforeID: newer})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, older, rows[0].ID)
	rows, err = query.ListDirectCorrections(testpkg.Ctx(t), auditlog.CorrectionFilter{})
	require.NoError(t, err)
	require.Empty(t, rows)
	require.Zero(t, observations[2].Queries, "zero limit must not turn into an unbounded read")
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	rows, err = query.ListDirectCorrections(tenant.WithTenantID(testpkg.Ctx(t), otherTenant), auditlog.CorrectionFilter{Limit: 10})
	require.NoError(t, err)
	require.Empty(t, rows)
	_, err = query.ListDirectCorrections(context.Background(), auditlog.CorrectionFilter{Limit: 1})
	require.Error(t, err)
	canceled, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()
	_, err = query.ListDirectCorrections(canceled, auditlog.CorrectionFilter{Limit: 1})
	require.ErrorIs(t, err, context.Canceled)
	_, err = NewCorrectionQueries(nil, func(auditlog.Observation) {})
	require.Error(t, err)
}
