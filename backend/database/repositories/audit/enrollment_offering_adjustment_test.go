package audit

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/audit"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnrollmentOfferingAdjustmentRepository_ListByRequestChildID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewEnrollmentOfferingAdjustmentRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Audit", "Adjustment", "1a")
	account := testpkg.CreateTestAccount(t, db, "adjustment-admin@example.test")
	_, requestID, childID := testpkg.CreateAuditAdjustmentChain(t, db)
	otherChildID := testpkg.CreateAuditAdjustmentChild(t, db, requestID)

	older := &audit.EnrollmentOfferingAdjustment{
		RequestID:      requestID,
		RequestChildID: childID,
		StudentID:      student.ID,
		ActorAccountID: account.ID,
		ActorRole:      "admin",
		Reason:         "older",
		Before:         []byte(`[]`),
		After:          []byte(`[{"name":"old"}]`),
		ChangedAt:      time.Date(2026, time.June, 20, 9, 0, 0, 0, time.UTC),
	}
	newer := &audit.EnrollmentOfferingAdjustment{
		RequestID:      requestID,
		RequestChildID: childID,
		StudentID:      student.ID,
		ActorAccountID: account.ID,
		ActorRole:      "admin",
		Reason:         "newer",
		Before:         []byte(`[]`),
		After:          []byte(`[{"name":"new"}]`),
		ChangedAt:      time.Date(2026, time.June, 20, 10, 0, 0, 0, time.UTC),
	}
	other := &audit.EnrollmentOfferingAdjustment{
		RequestID:      requestID,
		RequestChildID: otherChildID,
		StudentID:      student.ID,
		ActorAccountID: account.ID,
		ActorRole:      "admin",
		Reason:         "other child",
		Before:         []byte(`[]`),
		After:          []byte(`[]`),
		ChangedAt:      time.Date(2026, time.June, 20, 11, 0, 0, 0, time.UTC),
	}

	require.NoError(t, repo.Create(ctx, older))
	require.NoError(t, repo.Create(ctx, newer))
	require.NoError(t, repo.Create(ctx, other))

	rows, err := repo.ListByRequestChildID(ctx, childID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, newer.ID, rows[0].ID)
	assert.Equal(t, older.ID, rows[1].ID)
	assert.Equal(t, "newer", rows[0].Reason)
}

func TestEnrollmentOfferingAdjustmentRepository_ListByRequestChildID_RejectsMissingID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewEnrollmentOfferingAdjustmentRepository(NewRuntime(db, auditTestTenantID))

	rows, err := repo.ListByRequestChildID(testpkg.Ctx(t), 0)
	require.Error(t, err)
	assert.Nil(t, rows)
	assert.Contains(t, err.Error(), "request_child_id is required")
}

func TestEnrollmentOfferingAdjustmentRepository_ListByRequestChildID_QueryError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupClosableTestDB(t)
	repo := NewEnrollmentOfferingAdjustmentRepository(NewRuntime(db, auditTestTenantID))
	require.NoError(t, db.Close())

	rows, err := repo.ListByRequestChildID(testpkg.Ctx(t), 999)
	require.Error(t, err)
	assert.Nil(t, rows)
	assert.Contains(t, err.Error(), "list enrollment offering adjustments")
}

func TestEnrollmentOfferingAdjustmentRepository_ListDirectForTenant_QueryError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupClosableTestDB(t)
	repo := NewEnrollmentOfferingAdjustmentRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)
	require.NoError(t, db.Close())

	rows, err := repo.ListDirectForTenant(ctx, audit.DirectAdjustmentFilter{BeforeInstant: time.Now(), BeforeID: 1, Limit: 10})
	require.Error(t, err)
	assert.Nil(t, rows)
	assert.Contains(t, err.Error(), "list direct enrollment offering adjustments")
}

// The central history reads corrections through this keyset window (#2436):
// newest first, request-applied rows excluded, and a follow-up page that
// continues strictly before the last row of the previous one.
func TestEnrollmentOfferingAdjustmentRepository_ListDirectForTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewEnrollmentOfferingAdjustmentRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Direct", "Correction", "1a")
	account := testpkg.CreateTestAccount(t, db, "direct-correction-admin@example.test")
	_, requestID, childID := testpkg.CreateAuditAdjustmentChain(t, db)

	// A far-future base keeps this window clear of rows other tests leave in
	// the shared test database.
	base := time.Date(2099, time.March, 3, 12, 0, 0, 0, time.UTC)
	newRow := func(reason, source string, changedAt time.Time) *audit.EnrollmentOfferingAdjustment {
		return &audit.EnrollmentOfferingAdjustment{
			RequestID:      requestID,
			RequestChildID: childID,
			StudentID:      student.ID,
			ActorAccountID: account.ID,
			ActorRole:      "admin",
			Reason:         reason,
			Source:         source,
			Before:         []byte(`[]`),
			After:          []byte(`[]`),
			ChangedAt:      changedAt,
		}
	}
	newest := newRow("newest", audit.OfferingAdjustmentSourceDirect, base.Add(2*time.Hour))
	middle := newRow("middle", audit.OfferingAdjustmentSourceDirect, base.Add(time.Hour))
	oldest := newRow("oldest", audit.OfferingAdjustmentSourceDirect, base)
	applied := newRow("applied request", audit.OfferingAdjustmentSourceRequest, base.Add(3*time.Hour))
	unknown := newRow("legacy adjustment", audit.OfferingAdjustmentSourceUnknown, base.Add(4*time.Hour))
	for _, row := range []*audit.EnrollmentOfferingAdjustment{newest, middle, oldest, applied, unknown} {
		require.NoError(t, repo.Create(ctx, row))
	}

	// First page: newest first; request-applied and unknown legacy rows stay
	// out because neither is a verified direct correction.
	first, err := repo.ListDirectForTenant(ctx, audit.DirectAdjustmentFilter{Limit: 2})
	require.NoError(t, err)
	require.Len(t, first, 2)
	assert.Equal(t, []int64{newest.ID, middle.ID}, []int64{first[0].ID, first[1].ID})

	// Follow-up page continues strictly before the last consumed row.
	second, err := repo.ListDirectForTenant(ctx, audit.DirectAdjustmentFilter{BeforeInstant: first[1].ChangedAt, BeforeID: first[1].ID, Limit: 2})
	require.NoError(t, err)
	require.NotEmpty(t, second)
	assert.Equal(t, oldest.ID, second[0].ID)
	for _, row := range second {
		assert.NotEqual(t, applied.ID, row.ID, "request-applied rows never appear")
		assert.NotEqual(t, unknown.ID, row.ID, "unknown legacy rows never appear")
	}

	// The explicit repository predicate protects non-HTTP callers whose
	// database connection bypasses RLS.
	otherTenant := testpkg.NewTenantScope(t, db)
	otherTenantRows, err := repo.ListDirectForTenant(otherTenant.Context(), audit.DirectAdjustmentFilter{Limit: 2})
	require.NoError(t, err)
	assert.Empty(t, otherTenantRows)

	// A non-positive limit asks for nothing and hits no database at all.
	none, err := repo.ListDirectForTenant(ctx, audit.DirectAdjustmentFilter{})
	require.NoError(t, err)
	assert.Empty(t, none)
}
