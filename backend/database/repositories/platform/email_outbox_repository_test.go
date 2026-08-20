package platform_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func uniqueOutboxToken(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func setupOutboxRepoTest(t *testing.T) (*bun.DB, platformModels.EmailOutboxCleanupRepository, int64) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	return db, platformRepo.NewEmailOutboxRepository(db), tenantID
}

// runInTenantTx is local to this package so we don't cross-import the
// enrollment test helpers.
func runInTenantTx(t *testing.T, db *bun.DB, tenantID int64, fn func(ctx context.Context) error) error {
	t.Helper()
	return tenant.WithTenantTx(context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return fn(ctx)
	})
}

// runAsAdmin runs a closure under phoenix_admin (BYPASSRLS). The
// outbox worker uses this role for the cross-tenant ClaimDuePending
// query.
func runAsAdmin(t *testing.T, db *bun.DB, fn func(ctx context.Context) error) error {
	t.Helper()
	return tenant.WithAdminTx(context.Background(), db, func(ctx context.Context, _ bun.Tx) error {
		return fn(ctx)
	})
}

func wipeOutbox(db *bun.DB, tenantID int64, kindPrefix string) {
	_, _ = db.NewDelete().
		Table("platform.email_outbox").
		Where("tenant_id = ? AND kind LIKE ?", tenantID, kindPrefix+"%").
		Exec(context.Background())
}

// makeOutbox makes a fresh pending row with NextRetryAt = now so the
// claim path picks it up by default. Caller may override.
func makeOutbox(kind string) *platformModels.EmailOutbox {
	return &platformModels.EmailOutbox{
		Kind:        kind,
		Payload:     map[string]any{"recipient_email": "x@example.test"},
		Status:      platformModels.EmailOutboxStatusPending,
		NextRetryAt: time.Now(),
	}
}

// --- Create + FindByID + Validate -------------------------------------

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestEmailOutboxRepository_Create_PersistsAndReturnsID(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOutboxRepoTest(t)
	kind := uniqueOutboxToken("create")
	defer wipeOutbox(db, tenantID, kind)

	row := makeOutbox(kind)
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, row)
	})
	require.NoError(t, err)
	assert.NotZero(t, row.ID)
	assert.Equal(t, tenantID, row.TenantID)
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestEmailOutboxRepository_Create_RejectsInvalidRow(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOutboxRepoTest(t)
	row := makeOutbox("") // blank Kind → Validate fails
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, row)
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestEmailOutboxRepository_Create_IdempotencyKeySuppressesDuplicate(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOutboxRepoTest(t)
	kind := uniqueOutboxToken("idempotent")
	defer wipeOutbox(db, tenantID, kind)
	key := uniqueOutboxToken("decision-key")
	first := makeOutbox(kind)
	first.IdempotencyKey = &key
	second := makeOutbox(kind)
	second.IdempotencyKey = &key

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		if err := repo.Create(ctx, first); err != nil {
			return err
		}
		return repo.Create(ctx, second)
	}))

	count, err := db.NewSelect().
		Table("platform.email_outbox").
		Where("tenant_id = ?", tenantID).
		Where("idempotency_key = ?", key).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestEmailOutboxRepository_FindByID_HappyPath(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOutboxRepoTest(t)
	kind := uniqueOutboxToken("find")
	defer wipeOutbox(db, tenantID, kind)

	row := makeOutbox(kind)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, row)
	}))

	var got *platformModels.EmailOutbox
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.FindByID(ctx, row.ID)
		return fbErr
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, row.ID, got.ID)
	assert.Equal(t, kind, got.Kind)
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestEmailOutboxRepository_FindByID_NotFound(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOutboxRepoTest(t)
	var got *platformModels.EmailOutbox
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.FindByID(ctx, 9_999_999)
		return fbErr
	})
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "not found")
}

// --- ClaimDuePending --------------------------------------------------

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
// Deliberately NOT parallel: unscoped sweep — ClaimDuePending and the
// cancel-by-entity update run across tenants with a LIMIT, so a parallel
// test's rows compete for the same slots.
func TestEmailOutboxRepository_ClaimDuePending_ReturnsRowsAndFlipsToSending(t *testing.T) {

	// Insert two due pending rows + one future-retry row. Claim should
	// return exactly the two due ones, with their status flipped.
	db, repo, tenantID := setupOutboxRepoTest(t)
	kind := uniqueOutboxToken("claim")
	defer wipeOutbox(db, tenantID, kind)

	dueA := makeOutbox(kind)
	dueA.NextRetryAt = time.Now().Add(-1 * time.Hour)
	dueB := makeOutbox(kind)
	dueB.NextRetryAt = time.Now().Add(-30 * time.Minute)
	future := makeOutbox(kind)
	future.NextRetryAt = time.Now().Add(1 * time.Hour)

	for _, r := range []*platformModels.EmailOutbox{dueA, dueB, future} {
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, r)
		}))
	}

	var claimed []*platformModels.EmailOutbox
	err := runAsAdmin(t, db, func(ctx context.Context) error {
		var cErr error
		claimed, cErr = repo.ClaimDuePending(ctx, 10, time.Now())
		return cErr
	})
	require.NoError(t, err)

	// Filter to ours and check we got both due rows.
	claimedIDs := map[int64]bool{}
	for _, c := range claimed {
		if c.Kind == kind {
			claimedIDs[c.ID] = true
			assert.Equal(t, platformModels.EmailOutboxStatusSending, c.Status,
				"claimed row MUST come back with status='sending'")
		}
	}
	assert.True(t, claimedIDs[dueA.ID])
	assert.True(t, claimedIDs[dueB.ID])
	assert.False(t, claimedIDs[future.ID],
		"future-retry row MUST NOT be claimed (next_retry_at > now)")

	// Confirm the future row is still pending.
	var futureRow *platformModels.EmailOutbox
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		futureRow, fbErr = repo.FindByID(ctx, future.ID)
		return fbErr
	}))
	assert.Equal(t, platformModels.EmailOutboxStatusPending, futureRow.Status)
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
// Deliberately NOT parallel: unscoped sweep — ClaimDuePending and the
// cancel-by-entity update run across tenants with a LIMIT, so a parallel
// test's rows compete for the same slots.
func TestEmailOutboxRepository_ClaimDuePending_LimitCaps(t *testing.T) {

	db, repo, tenantID := setupOutboxRepoTest(t)
	kind := uniqueOutboxToken("limit")
	defer wipeOutbox(db, tenantID, kind)

	// Insert 5 due rows, claim with limit=2 → only 2 come back.
	for i := 0; i < 5; i++ {
		r := makeOutbox(kind)
		r.NextRetryAt = time.Now().Add(-1 * time.Hour)
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, r)
		}))
	}

	var claimed []*platformModels.EmailOutbox
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var cErr error
		claimed, cErr = repo.ClaimDuePending(ctx, 2, time.Now())
		return cErr
	}))
	ours := 0
	for _, c := range claimed {
		if c.Kind == kind {
			ours++
		}
	}
	assert.Equal(t, 2, ours, "limit=2 must return at most 2 rows of ours")
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestEmailOutboxRepository_ClaimDuePending_AlreadySendingNotReClaimed(t *testing.T) {
	t.Parallel()

	// A row already in 'sending' state must NOT be re-claimed — the
	// status filter is explicit and FOR UPDATE SKIP LOCKED prevents
	// duplicate work between concurrent workers.
	db, repo, tenantID := setupOutboxRepoTest(t)
	kind := uniqueOutboxToken("sending")
	defer wipeOutbox(db, tenantID, kind)

	r := makeOutbox(kind)
	r.NextRetryAt = time.Now().Add(-1 * time.Hour)
	r.Status = platformModels.EmailOutboxStatusSending
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, r)
	}))

	var claimed []*platformModels.EmailOutbox
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var cErr error
		claimed, cErr = repo.ClaimDuePending(ctx, 10, time.Now())
		return cErr
	}))
	for _, c := range claimed {
		assert.NotEqual(t, r.ID, c.ID,
			"row already in 'sending' MUST NOT be re-claimed (status filter)")
	}
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
// Deliberately NOT parallel: unscoped sweep — ClaimDuePending and the
// cancel-by-entity update run across tenants with a LIMIT, so a parallel
// test's rows compete for the same slots.
func TestEmailOutboxRepository_ClaimDuePending_OrdersByNextRetryAsc(t *testing.T) {

	// FIFO-ish ordering — oldest next_retry_at first so retries don't
	// starve.
	db, repo, tenantID := setupOutboxRepoTest(t)
	kind := uniqueOutboxToken("order")
	defer wipeOutbox(db, tenantID, kind)

	older := makeOutbox(kind)
	older.NextRetryAt = time.Now().Add(-2 * time.Hour)
	newer := makeOutbox(kind)
	newer.NextRetryAt = time.Now().Add(-1 * time.Hour)
	for _, r := range []*platformModels.EmailOutbox{newer, older} { // insert out of order
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, r)
		}))
	}

	var claimed []*platformModels.EmailOutbox
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var cErr error
		claimed, cErr = repo.ClaimDuePending(ctx, 10, time.Now())
		return cErr
	}))
	var ours []*platformModels.EmailOutbox
	for _, c := range claimed {
		if c.Kind == kind {
			ours = append(ours, c)
		}
	}
	require.GreaterOrEqual(t, len(ours), 2)
	// older.ID was created first chronologically but next_retry_at is
	// earlier, so it must appear FIRST in the claimed slice.
	assert.Equal(t, older.ID, ours[0].ID,
		"oldest next_retry_at must be claimed first to avoid starvation")
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestEmailOutboxRepository_ClaimDuePending_ZeroLimitDefaultsTo25(t *testing.T) {
	t.Parallel()

	// `if limit <= 0 { limit = 25 }` — caller passing 0 must get the
	// default budget, not zero results.
	db, repo, tenantID := setupOutboxRepoTest(t)
	kind := uniqueOutboxToken("zeroLimit")
	defer wipeOutbox(db, tenantID, kind)

	r := makeOutbox(kind)
	// Far enough back to sort ahead of every other pending row in the clone:
	// the claim is tenant-less and takes the 25 oldest due rows, so a row that
	// is merely "due" competes with whatever other tests left pending (#2419).
	r.NextRetryAt = time.Now().Add(-365 * 24 * time.Hour)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, r)
	}))

	var claimed []*platformModels.EmailOutbox
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var cErr error
		claimed, cErr = repo.ClaimDuePending(ctx, 0, time.Now())
		return cErr
	}))
	found := false
	for _, c := range claimed {
		if c.ID == r.ID {
			found = true
		}
	}
	assert.True(t, found, "limit=0 must default to the budget (25), not return 0 rows")
}

// --- LockSending -------------------------------------------------------

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestEmailOutboxRepository_LockSending_TrueForSendingRow(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOutboxRepoTest(t)
	kind := uniqueOutboxToken("locksending")
	defer wipeOutbox(db, tenantID, kind)

	r := makeOutbox(kind)
	r.Status = platformModels.EmailOutboxStatusSending
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, r)
	}))

	var claimed bool
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var lErr error
		claimed, lErr = repo.LockSending(ctx, r.ID)
		return lErr
	}))
	assert.True(t, claimed, "a row still in 'sending' must lock as claimed")
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
// Deliberately NOT parallel: unscoped sweep — ClaimDuePending and the
// cancel-by-entity update run across tenants with a LIMIT, so a parallel
// test's rows compete for the same slots.
func TestEmailOutboxRepository_LockSending_FalseWhenGoneOrNotSending(t *testing.T) {

	db, repo, tenantID := setupOutboxRepoTest(t)
	kind := uniqueOutboxToken("lockgone")
	defer wipeOutbox(db, tenantID, kind)

	pending := makeOutbox(kind) // still 'pending' — not a valid claim
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, pending)
	}))

	for name, id := range map[string]int64{
		"deleted row":     9_999_999,
		"non-sending row": pending.ID,
	} {
		var claimed bool
		require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
			var lErr error
			claimed, lErr = repo.LockSending(ctx, id)
			return lErr
		}), name)
		assert.False(t, claimed, "%s must report not claimed, without error", name)
	}
}

// --- MarkSent / MarkRetry / MarkFailed --------------------------------

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestEmailOutboxRepository_MarkSent_HappyPath(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOutboxRepoTest(t)
	kind := uniqueOutboxToken("marksent")
	defer wipeOutbox(db, tenantID, kind)

	r := makeOutbox(kind)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, r)
	}))

	sentAt := time.Now().Truncate(time.Second)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.MarkSent(ctx, r.ID, sentAt)
	}))

	var got *platformModels.EmailOutbox
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.FindByID(ctx, r.ID)
		return fbErr
	}))
	assert.Equal(t, platformModels.EmailOutboxStatusSent, got.Status)
	require.NotNil(t, got.SentAt)
	assert.WithinDuration(t, sentAt, *got.SentAt, time.Second,
		"sent_at must round-trip with no clock drift")
	assert.Nil(t, got.LastError, "MarkSent must CLEAR last_error")
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestEmailOutboxRepository_MarkSent_MissingIDErrors(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOutboxRepoTest(t)
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.MarkSent(ctx, 9_999_999, time.Now())
	})
	require.Error(t, err)
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestEmailOutboxRepository_MarkRetry_HappyPath(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOutboxRepoTest(t)
	kind := uniqueOutboxToken("markretry")
	defer wipeOutbox(db, tenantID, kind)

	r := makeOutbox(kind)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, r)
	}))

	retryAt := time.Now().Add(5 * time.Minute).Truncate(time.Second)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.MarkRetry(ctx, r.ID, 1, "smtp 421", retryAt)
	}))

	var got *platformModels.EmailOutbox
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.FindByID(ctx, r.ID)
		return fbErr
	}))
	assert.Equal(t, platformModels.EmailOutboxStatusPending, got.Status,
		"MarkRetry returns the row to 'pending' so the next tick picks it up")
	assert.Equal(t, 1, got.Attempts)
	require.NotNil(t, got.LastError)
	assert.Equal(t, "smtp 421", *got.LastError)
	assert.WithinDuration(t, retryAt, got.NextRetryAt, time.Second)
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestEmailOutboxRepository_MarkRetry_MissingIDErrors(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOutboxRepoTest(t)
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.MarkRetry(ctx, 9_999_999, 1, "boom", time.Now())
	})
	require.Error(t, err)
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestEmailOutboxRepository_MarkFailed_TerminalAttemptCountPersisted(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOutboxRepoTest(t)
	kind := uniqueOutboxToken("markfailed")
	defer wipeOutbox(db, tenantID, kind)

	r := makeOutbox(kind)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, r)
	}))

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.MarkFailed(ctx, r.ID, 5, "permanent failure")
	}))

	var got *platformModels.EmailOutbox
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.FindByID(ctx, r.ID)
		return fbErr
	}))
	assert.Equal(t, platformModels.EmailOutboxStatusFailed, got.Status,
		"MarkFailed sets terminal status")
	assert.Equal(t, 5, got.Attempts, "final attempts count persists for ops visibility")
	require.NotNil(t, got.LastError)
	assert.Equal(t, "permanent failure", *got.LastError)
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestEmailOutboxRepository_MarkFailed_MissingIDErrors(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOutboxRepoTest(t)
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.MarkFailed(ctx, 9_999_999, 1, "boom")
	})
	require.Error(t, err)
}

// --- FindByRelatedEntity ---------------------------------------------

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestEmailOutboxRepository_FindByRelatedEntity_FiltersByTypeAndID(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOutboxRepoTest(t)
	kind := uniqueOutboxToken("related")
	defer wipeOutbox(db, tenantID, kind)

	// Two rows referencing the same enrollment_request, one referencing
	// a different one, one referencing a different type entirely.
	relatedID := int64(424242)
	otherID := int64(919191)
	relatedType := platformModels.EmailRelatedTypeEnrollmentRequest
	otherType := platformModels.EmailRelatedTypeGuardianInvitation

	makeRelated := func(rType *string, rID *int64) *platformModels.EmailOutbox {
		r := makeOutbox(kind)
		r.RelatedEntityType = rType
		r.RelatedEntityID = rID
		return r
	}

	for _, r := range []*platformModels.EmailOutbox{
		makeRelated(&relatedType, &relatedID),
		makeRelated(&relatedType, &relatedID),
		makeRelated(&relatedType, &otherID),
		makeRelated(&otherType, &relatedID),
	} {
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, r)
		}))
	}

	var list []*platformModels.EmailOutbox
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.FindByRelatedEntity(ctx, relatedType, relatedID)
		return lErr
	}))
	count := 0
	for _, r := range list {
		if r.Kind != kind {
			continue
		}
		count++
		require.NotNil(t, r.RelatedEntityType)
		require.NotNil(t, r.RelatedEntityID)
		assert.Equal(t, relatedType, *r.RelatedEntityType)
		assert.Equal(t, relatedID, *r.RelatedEntityID)
	}
	assert.Equal(t, 2, count,
		"FindByRelatedEntity must filter on BOTH type and id (saw %d, want 2)", count)
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestEmailOutboxRepository_FindByRelatedEntity_EmptyResultNoError(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOutboxRepoTest(t)
	var list []*platformModels.EmailOutbox
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.FindByRelatedEntity(ctx,
			platformModels.EmailRelatedTypeEnrollmentRequest, 9_999_999)
		return lErr
	})
	require.NoError(t, err)
	assert.Empty(t, list)
}

// --- CancelPendingByRelatedEntity ------------------------------------

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
// Deliberately NOT parallel: unscoped sweep — ClaimDuePending and the
// cancel-by-entity update run across tenants with a LIMIT, so a parallel
// test's rows compete for the same slots.
func TestEmailOutboxRepository_CancelPendingByRelatedEntity_FailsOnlyPendingForEntity(t *testing.T) {

	db, repo, tenantID := setupOutboxRepoTest(t)
	kind := uniqueOutboxToken("cancel")
	defer wipeOutbox(db, tenantID, kind)

	relatedType := platformModels.EmailKindParentAnnouncement
	targetID := int64(717171)
	otherID := int64(818181)

	makeRelated := func(rID int64, status string) *platformModels.EmailOutbox {
		r := makeOutbox(kind)
		rt := relatedType
		id := rID
		r.RelatedEntityType = &rt
		r.RelatedEntityID = &id
		r.Status = status
		return r
	}

	// Two pending rows for the target entity (should be cancelled), a
	// 'sending' row for the target entity (already claimed → left alone), and a
	// pending row for a different entity (out of scope).
	pendingA := makeRelated(targetID, platformModels.EmailOutboxStatusPending)
	pendingB := makeRelated(targetID, platformModels.EmailOutboxStatusPending)
	sending := makeRelated(targetID, platformModels.EmailOutboxStatusSending)
	otherEntity := makeRelated(otherID, platformModels.EmailOutboxStatusPending)
	for _, r := range []*platformModels.EmailOutbox{pendingA, pendingB, sending, otherEntity} {
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, r)
		}))
	}

	var cancelled int64
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var cErr error
		cancelled, cErr = repo.CancelPendingByRelatedEntity(ctx, relatedType, targetID, "announcement retracted before send")
		return cErr
	}))
	assert.EqualValues(t, 2, cancelled, "only the two pending rows for the entity must be cancelled")

	statusOf := func(id int64) *platformModels.EmailOutbox {
		var got *platformModels.EmailOutbox
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			var fbErr error
			got, fbErr = repo.FindByID(ctx, id)
			return fbErr
		}))
		return got
	}

	for _, id := range []int64{pendingA.ID, pendingB.ID} {
		row := statusOf(id)
		assert.Equal(t, platformModels.EmailOutboxStatusFailed, row.Status,
			"pending row for the entity must be flipped to failed so the worker skips it")
		require.NotNil(t, row.LastError)
		assert.Equal(t, "announcement retracted before send", *row.LastError)
	}

	assert.Equal(t, platformModels.EmailOutboxStatusSending, statusOf(sending.ID).Status,
		"a row already claimed by the worker ('sending') must NOT be cancelled")
	assert.Equal(t, platformModels.EmailOutboxStatusPending, statusOf(otherEntity.ID).Status,
		"a pending row for a DIFFERENT entity must be untouched")
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestEmailOutboxRepository_CancelPendingByRelatedEntity_NoMatchesReturnsZero(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOutboxRepoTest(t)
	var cancelled int64
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var cErr error
		cancelled, cErr = repo.CancelPendingByRelatedEntity(ctx,
			platformModels.EmailKindParentAnnouncement, 9_999_999, "reason")
		return cErr
	})
	require.NoError(t, err)
	assert.Zero(t, cancelled)
}
