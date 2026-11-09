package api

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	securityRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/securityruntime/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestBankDataRekeyKeepsHistoryAndTraegerAndResumes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.NewDatabase(t, db).UnitOfWork())
	own := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Rekey", Slug: fmt.Sprintf("rekey-%d", testpkg.UniqueTestTenantID(t))}).ID
	other := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Other", Slug: fmt.Sprintf("other-rekey-%d", testpkg.UniqueTestTenantID(t))}).ID
	legacyKeys, bankKeys := testpkg.NewTestHealthDataKeys(), testpkg.NewTestHealthDataKeys()
	legacy, err := securityRuntimeCompose.NewFieldCipher(legacyKeys)
	require.NoError(t, err)
	cipher, err := securityRuntimeCompose.NewBankDataCipher(bankKeys, legacyKeys)
	require.NoError(t, err)
	plainIBAN, plainHolder := "DE89370400440532013000", "Synthetic account holder"
	seal := func(org int64, ref, field, value string) []byte {
		t.Helper()
		sealed, err := legacy.Seal([]byte(value), []byte(fmt.Sprintf("parent_billing.sepa_mandates.%s|%d|%s", field, org, ref)))
		require.NoError(t, err)
		return sealed
	}
	// Both active and revoked rows, and a foreign row, use real organization
	// fixtures. Foreign keys are irrelevant to encryption and disabled only
	// during this isolated fixture transaction, as in the mandate scope test.
	require.NoError(t, db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, "SET LOCAL session_replication_role = replica")
		if err != nil {
			return err
		}
		for i, org := range []int64{own, own, other} {
			ref := fmt.Sprintf("REKEY-%d", i)
			var revokedOn, revokedAt any
			if i == 0 {
				revokedOn, revokedAt = "2026-10-01", "2026-10-01T12:00:00Z"
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.sepa_mandates
    (organization_id, family_email, care_contract_id, reference, signed_on, kind, sequence, creditor_id,
     account_holder_sealed, iban_sealed, iban_masked, source, revoked_on, revoked_at)
    VALUES (?, ?, ?, ?, '2026-09-01', 'recurring', 'first', 'DE98ZZZ09999999999', ?, ?, 'masked', 'import', ?::date, ?::timestamptz)`,
				org, fmt.Sprintf("rekey%d@example.org", i), testpkg.UniqueTestTenantID(t), ref,
				seal(org, ref, "account_holder", plainHolder), seal(org, ref, "iban", plainIBAN), revokedOn, revokedAt)
			if err != nil {
				return err
			}
		}
		return nil
	}))
	var historyBefore, foreignBefore string
	require.NoError(t, db.NewRaw(`SELECT jsonb_agg(to_jsonb(m) - ARRAY['iban_sealed','account_holder_sealed'] ORDER BY id)::text FROM parent_billing.sepa_mandates m WHERE organization_id = ?`, own).Scan(ctx, &historyBefore))
	require.NoError(t, db.NewRaw(`SELECT to_jsonb(m)::text FROM parent_billing.sepa_mandates m WHERE organization_id = ?`, other).Scan(ctx, &foreignBefore))
	var logged bankRekeyLogBuffer
	testpkg.NewDatabase(t, db).LogQueries(slog.New(slog.NewJSONHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug})))
	job := parentBillingCompose.NewBankDataRekey(cipher)
	first, err := job.Batch(ctx, own, "mandates", 0, 1)
	require.NoError(t, err)
	require.Equal(t, 1, first.Changed)
	repeated, err := job.Batch(ctx, own, "mandates", 0, 1)
	require.NoError(t, err)
	require.Zero(t, repeated.Changed)
	second, err := job.Batch(ctx, own, "mandates", first.LastID, 1)
	require.NoError(t, err)
	require.Equal(t, 1, second.Changed)
	finished, err := job.Batch(ctx, own, "mandates", second.LastID, 1)
	require.NoError(t, err)
	require.Zero(t, finished.Scanned)
	independent, err := securityRuntimeCompose.NewBankDataCipher(bankKeys, "")
	require.NoError(t, err)
	var rows []struct {
		Reference string
		IBAN      []byte `bun:"iban_sealed"`
		Holder    []byte `bun:"account_holder_sealed"`
	}
	require.NoError(t, db.NewRaw(`SELECT reference, iban_sealed, account_holder_sealed FROM parent_billing.sepa_mandates WHERE organization_id = ?`, own).Scan(ctx, &rows))
	for _, row := range rows {
		for field, value := range map[string][]byte{"iban": row.IBAN, "account_holder": row.Holder} {
			aad := []byte(fmt.Sprintf("parent_billing.sepa_mandates.%s|%d|%s", field, own, row.Reference))
			opened, err := independent.Open(value, aad)
			require.NoError(t, err)
			if field == "iban" {
				require.Equal(t, plainIBAN, string(opened))
			} else {
				require.Equal(t, plainHolder, string(opened))
			}
			_, err = legacy.Open(value, aad)
			require.Error(t, err)
		}
	}
	require.NotContains(t, logged.String(), plainIBAN)
	require.NotContains(t, logged.String(), plainHolder)
	require.NotContains(t, logged.String(), legacyKeys)
	require.NotContains(t, logged.String(), bankKeys)
	var historyAfter, foreignAfter string
	require.NoError(t, db.NewRaw(`SELECT jsonb_agg(to_jsonb(m) - ARRAY['iban_sealed','account_holder_sealed'] ORDER BY id)::text FROM parent_billing.sepa_mandates m WHERE organization_id = ?`, own).Scan(ctx, &historyAfter))
	require.Equal(t, historyBefore, historyAfter)
	require.NoError(t, db.NewRaw(`SELECT to_jsonb(m)::text FROM parent_billing.sepa_mandates m WHERE organization_id = ?`, other).Scan(ctx, &foreignAfter))
	require.Equal(t, foreignBefore, foreignAfter)
	assertBankRekeySerializesWriter(t, db, own, cipher, second.LastID)
	assertBankRekeyRollsBackBatch(t, ctx, db, own, second.LastID, bankKeys)

	// A new ring without a legacy key cannot process the foreign row. Its
	// error is deliberately content-free, even on crypto/SQL failure.
	failedJob := parentBillingCompose.NewBankDataRekey(independent)
	failed, err := failedJob.Batch(ctx, other, "mandates", 0, 100)
	require.Error(t, err)
	require.Zero(t, failed.Changed)
	require.NotContains(t, err.Error(), plainIBAN)
	require.NotContains(t, err.Error(), plainHolder)
	require.NotContains(t, err.Error(), legacyKeys)
	require.NoError(t, db.NewRaw(`SELECT to_jsonb(m)::text FROM parent_billing.sepa_mandates m WHERE organization_id = ?`, other).Scan(ctx, &foreignAfter))
	require.Equal(t, foreignBefore, foreignAfter)
}

// blockingRewrapper pauses after the batch has taken its row locks.
type blockingBankRewrapper struct {
	parentbilling.BankDataRewrapper
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockingBankRewrapper) Rewrap(value, aad []byte) ([]byte, error) {
	b.once.Do(func() { close(b.entered); <-b.release })
	return b.BankDataRewrapper.Rewrap(value, aad)
}

func assertBankRekeySerializesWriter(t *testing.T, db *bun.DB, organizationID int64,
	cipher interface {
		parentbilling.BankDataRewrapper
		parentbilling.BankDataCipher
	}, activeID int64,
) {
	t.Helper()
	blocker := &blockingBankRewrapper{BankDataRewrapper: cipher, entered: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(blocker.release) }) }
	defer release()
	ctx, cancel := context.WithTimeout(tenant.WithUnitOfWork(context.Background(), testpkg.NewDatabase(t, db).UnitOfWork()), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := parentBillingCompose.NewBankDataRekey(blocker).Batch(ctx, organizationID, "mandates", 0, 100)
		done <- err
	}()
	select {
	case <-blocker.entered:
	case <-ctx.Done():
		t.Fatal("rekey did not reach the locked row")
	}
	var reference string
	require.NoError(t, db.NewRaw(`SELECT reference FROM parent_billing.sepa_mandates WHERE organization_id = ? AND id = ?`, organizationID, activeID).Scan(ctx, &reference))
	aad := []byte(fmt.Sprintf("parent_billing.sepa_mandates.account_holder|%d|%s", organizationID, reference))
	updated, err := cipher.Seal([]byte("Concurrent synthetic holder"), aad)
	require.NoError(t, err)
	writer := make(chan error, 1)
	go func() {
		_, err := db.NewRaw(`UPDATE parent_billing.sepa_mandates /* bank_rekey_concurrent_writer */ SET account_holder_sealed = ? WHERE organization_id = ? AND id = ?`, updated, organizationID, activeID).Exec(ctx)
		writer <- err
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname = current_database()
   AND wait_event_type = 'Lock' AND query LIKE 'UPDATE parent_billing.sepa_mandates /* bank_rekey_concurrent_writer */%')`).Scan(ctx, &waiting)
		return err == nil && waiting
	}, 3*time.Second, 10*time.Millisecond, "the writer must wait for the rekey's row lock")
	release()
	require.NoError(t, <-done)
	require.NoError(t, <-writer)
	var kept []byte
	require.NoError(t, db.NewRaw(`SELECT account_holder_sealed FROM parent_billing.sepa_mandates WHERE organization_id = ? AND id = ?`, organizationID, activeID).Scan(ctx, &kept))
	plain, err := cipher.Open(kept, aad)
	require.NoError(t, err)
	require.Equal(t, "Concurrent synthetic holder", string(plain), "rekey must not overwrite the concurrent write")
}

type bankRekeyLogBuffer struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (b *bankRekeyLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.Write(p)
}
func (b *bankRekeyLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.String()
}

func assertBankRekeyRollsBackBatch(t *testing.T, ctx context.Context, db *bun.DB, organizationID, activeID int64, bankKeys string) {
	t.Helper()
	_, err := db.NewRaw(`UPDATE parent_billing.sepa_mandates SET account_holder_sealed = ? WHERE organization_id = ? AND id = ?`, []byte("invalid sealed holder"), organizationID, activeID).Exec(ctx)
	require.NoError(t, err)
	var before, after string
	query := `SELECT jsonb_agg(to_jsonb(m) ORDER BY id)::text FROM parent_billing.sepa_mandates m WHERE organization_id = ?`
	require.NoError(t, db.NewRaw(query, organizationID).Scan(ctx, &before))
	nextKeys := "next" + testpkg.NewTestHealthDataKeys()
	cipher, err := securityRuntimeCompose.NewBankDataCipher(nextKeys+","+bankKeys, "")
	require.NoError(t, err)
	batch, err := parentBillingCompose.NewBankDataRekey(cipher).Batch(ctx, organizationID, "mandates", 0, 100)
	require.Error(t, err)
	require.Zero(t, batch.Changed)
	require.NoError(t, db.NewRaw(query, organizationID).Scan(ctx, &after))
	require.Equal(t, before, after, "a later invalid holder rolls back the earlier successfully rewritten mandate")
	var replicationRole string
	require.NoError(t, db.NewRaw("SHOW session_replication_role").Scan(ctx, &replicationRole))
	require.Equal(t, "origin", replicationRole, "trigger bypass must not survive the transaction")
}
