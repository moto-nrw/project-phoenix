package compose

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// renewingBillingCounts renews the Worker lease in its own transaction while
// the first capture counts, as the Worker's renewal loop does while a job
// runs. The capture's snapshot is taken by then, so its commit fence meets a
// lease row that changed after the snapshot.
type renewingBillingCounts struct {
	db       *bun.DB
	lease    string
	schoolID int64
	renewals atomic.Int32
}

func (r *renewingBillingCounts) CountActiveStudentsByTenant(ctx context.Context, _ time.Time) (map[int64]int, error) {
	if r.renewals.Add(1) == 1 {
		if _, err := r.db.NewRaw(`UPDATE platform.worker_leases
SET lease_until = clock_timestamp() + interval '1 minute', updated_at = clock_timestamp()
WHERE lease_name = ?`, r.lease).Exec(context.WithoutCancel(ctx)); err != nil {
			return nil, err
		}
	}
	return map[int64]int{r.schoolID: 4}, nil
}

func (r *renewingBillingCounts) CountActiveTerminalsByTenant(context.Context) (map[int64]int, error) {
	return map[int64]int{r.schoolID: 1}, nil
}

// leaseFence installs the Worker's commit guard for one lease term: the same
// platform.assert_worker_lease call the Worker's lease store makes as the
// last statement of every job transaction.
func leaseFence(ctx context.Context, lease string, token int64, asserts *atomic.Int32) context.Context {
	return tenant.WithCommitGuard(ctx, func(txCtx context.Context) error {
		asserts.Add(1)
		transaction, _ := tenant.TransactionFromContext(txCtx)
		_, err := transaction.(bun.Tx).NewRaw(`SELECT platform.assert_worker_lease(?, 'billing-test-holder', ?, interval '1 second')`, lease, token).Exec(txCtx)
		return err
	})
}

func insertWorkerLease(t *testing.T, db *bun.DB, lease string) {
	t.Helper()
	_, err := db.NewRaw(`INSERT INTO platform.worker_leases (lease_name, holder_id, fencing_token, lease_until)
VALUES (?, 'billing-test-holder', 1, clock_timestamp() + interval '1 minute')`, lease).Exec(context.Background())
	require.NoError(t, err)
}

// TestBillingCaptureSurvivesALeaseRenewal pins #5715: the Worker renews its
// lease while the capture runs. The capture's repeatable-read fence then
// fails with a serialization conflict; the capture replays in the same run
// instead of failing until the next tick.
func TestBillingCaptureSurvivesALeaseRenewal(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	lease := "billing-test-" + t.Name()
	insertWorkerLease(t, db, lease)
	counts := &renewingBillingCounts{db: db, lease: lease, schoolID: schoolID}
	billing, err := NewBilling(BillingDependencies{Counts: counts, Now: func() time.Time { return berlinTime(2099, time.June, 1, 12, 0) }})
	require.NoError(t, err)
	ctx := adminCtx(t, db)
	_, err = billing.SetBillingKeyDay(ctx, 10, testpkg.UniqueTestTenantID(t))
	require.NoError(t, err)

	var asserts atomic.Int32
	written, err := billing.RecordDueBillingKeyDates(leaseFence(ctx, lease, 1, &asserts), berlinTime(2099, time.June, 10, 6, 0))
	require.NoError(t, err, "a lease renewal during the capture is no capture failure")
	assert.Positive(t, written)
	assert.Equal(t, int32(2), asserts.Load(), "the conflicted attempt is replayed once")

	rows, err := billing.ListBillingKeyDateCounts(ctx)
	require.NoError(t, err)
	row := countOf(t, rows, schoolID, "2099-06-01")
	assert.Equal(t, 4, row.ActiveStudents)
	assert.Equal(t, 1, row.ActiveTerminals)
}

// TestBillingCaptureStopsWhenTheLeaseIsLost pins that the replay does not
// weaken the fence: a term that no longer holds its lease aborts the capture
// at once, without a replay and without a row.
func TestBillingCaptureStopsWhenTheLeaseIsLost(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	lease := "billing-test-" + t.Name()
	insertWorkerLease(t, db, lease)
	counts := &stubBillingCounts{students: map[int64]int{schoolID: 4}, terminals: map[int64]int{schoolID: 1}}
	billing, err := NewBilling(BillingDependencies{Counts: counts, Now: func() time.Time { return berlinTime(2099, time.June, 1, 12, 0) }})
	require.NoError(t, err)
	ctx := adminCtx(t, db)
	_, err = billing.SetBillingKeyDay(ctx, 10, testpkg.UniqueTestTenantID(t))
	require.NoError(t, err)

	var asserts atomic.Int32
	_, err = billing.RecordDueBillingKeyDates(leaseFence(ctx, lease, 2, &asserts), berlinTime(2099, time.June, 10, 6, 0))
	require.ErrorContains(t, err, "is not held by fencing token 2")
	assert.Equal(t, int32(1), asserts.Load(), "a lost lease is not replayed")

	rows, err := billing.ListBillingKeyDateCounts(ctx)
	require.NoError(t, err)
	assert.Empty(t, rows)
}
