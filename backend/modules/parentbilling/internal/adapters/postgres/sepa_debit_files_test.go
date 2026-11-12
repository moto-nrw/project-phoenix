package postgres

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The stored PAIN.008 files (#6314, ADR 0187) against member deletions in
// other transactions, the evidence rule of a submitted original and the
// privileges of the request role. The rows are committed: the defects are
// between two transactions.

// fileScope is a committed released run with two pending debits of one
// family's mandate and a second contract with a mandate of its own.
type fileScope struct {
	debitScope
	db            *bun.DB
	ctx           context.Context
	unique        int64
	second        int64
	otherContract int64
	otherMandate  int64
}

func newFileScope(t *testing.T, name string) fileScope {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	scope := newMandateScope(t, db, name)
	school := testpkg.CreateTestSchoolInOrganization(t, db, scope.organization, name+" OGS")
	// Contracts go first (with their mandates and debits), then the runs
	// and the creditor.
	testpkg.OwnTraegerContributionRules(t, db, scope.organization)
	testpkg.OwnTraegerCareContracts(t, db, scope.organization)
	ctx := context.Background()
	unique := testpkg.UniqueTestTenantID(t)
	f := fileScope{db: db, unique: unique, ctx: testpkg.ContextWithTenantRuntime(ctx, testpkg.TenantRuntime(t, db))}
	f.debitScope = seedDebitScope(t, ctx, db, scope, school, "DE98ZZZ09999999999", fmt.Sprintf("FILE-%d", unique))
	// A sibling's contract: one invoice per contract and month.
	var contract int64
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id, status, sent_at)
		VALUES (?, ?, ?, 'versendet', NOW()) RETURNING id`, f.organization, school, testpkg.UniqueTestTenantID(t)).Scan(ctx, &contract))
	var invoice int64
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_invoices (organization_id, run_id, number, number_year, number_sequence,
		issued_on, billing_month, care_contract_id, school_id, school_name, child_first_name, child_last_name, recipient_first_name,
		recipient_last_name, recipient_email, month_rule, total_cents)
		VALUES (?, ?, ?, 2029, 2, DATE '2029-05-20', DATE '2029-06-01', ?, ?, 'OGS', 'Lia', 'Beispiel', 'Eva', 'Beispiel',
			'eva@example.org', 'pro_rata', 1200) RETURNING id`,
		f.organization, f.run, fmt.Sprintf("RE-FILE-%d-2", unique), contract, school).Scan(ctx, &invoice))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.sepa_debits (organization_id, mandate_id, care_contract_id, run_kind, run_id,
		care_invoice_id, invoice_number, amount_cents, due_on, sequence_type, creditor_id, mandate_reference, mandate_signed_on,
		account_holder_sealed, iban_sealed)
		VALUES (?, ?, ?, 'care', ?, ?, ?, 1200, DATE '2029-06-04', 'FRST', 'DE98ZZZ09999999999', ?, DATE '2026-09-01',
			'\x02'::bytea, '\x01'::bytea) RETURNING id`,
		f.organization, f.mandate, contract, f.run, invoice, fmt.Sprintf("RE-FILE-%d-2", unique), fmt.Sprintf("FILE-%d", unique)).Scan(ctx, &f.second))
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id, status)
		VALUES (?, ?, ?, 'entwurf') RETURNING id`, f.organization, school, testpkg.UniqueTestTenantID(t)).Scan(ctx, &f.otherContract))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.sepa_mandates (organization_id, family_email, care_contract_id, reference,
		signed_on, kind, sequence, creditor_id, account_holder_sealed, iban_sealed, iban_masked, source)
		VALUES (?, ?, ?, ?, DATE '2026-09-01', 'recurring', 'first', 'DE98ZZZ09999999999', '\x02'::bytea, '\x01'::bytea, 'DE•• 30 00', 'office')
		RETURNING id`, f.organization, "zweite."+f.family, f.otherContract, fmt.Sprintf("FILE-%d-B", unique)).Scan(ctx, &f.otherMandate))
	return f
}

// keep stores a file row as the owner, without the store: submitted marks
// it the original of a submitted message, age moves its download back.
func (f fileScope) keep(t *testing.T, message string, submitted bool, age time.Duration, debits, mandates []int64) {
	t.Helper()
	submittedAt := "NULL"
	if submitted {
		submittedAt = "clock_timestamp()"
	}
	_, err := f.db.ExecContext(context.Background(), `INSERT INTO parent_billing.sepa_debit_files (organization_id, message_id, run_kind,
		run_id, file_sealed, file_sha256, debit_ids, mandate_ids, downloaded_at, submitted_at)
		VALUES (?, ?, 'care', ?, '\x01'::bytea, ?, ?::bigint[], ?::bigint[], clock_timestamp() - ?::interval, `+submittedAt+`)`,
		f.organization, message, f.run, strings.Repeat("a", 64), members(debits), members(mandates),
		fmt.Sprintf("%d seconds", int64(age.Seconds())))
	require.NoError(t, err)
}

func (f fileScope) kept(t *testing.T) []string {
	t.Helper()
	var messages []string
	require.NoError(t, f.db.NewRaw(`SELECT message_id FROM parent_billing.sepa_debit_files WHERE organization_id = ? ORDER BY message_id`,
		f.organization).Scan(context.Background(), &messages))
	return messages
}

func (f fileScope) file(message string) parentbilling.StoredSEPADebitFile {
	return parentbilling.StoredSEPADebitFile{MessageID: message, RunKind: "care", RunID: f.run, Sealed: []byte{1},
		SHA256: strings.Repeat("b", 64), CreditorID: "DE98ZZZ09999999999", DebitIDs: []int64{f.debit, f.second},
		NoticeIDs: []int64{}, MandateIDs: []int64{f.mandate}}
}

// waitForLock waits until a statement that carries marker waits for a lock.
func waitForLock(t *testing.T, db *bun.DB, marker string, done <-chan error) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			t.Fatalf("the statement %s finished without waiting for the lock: %v", marker, err)
		default:
		}
		var waiting int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND query LIKE ?`,
			"%"+marker+"%").Scan(context.Background(), &waiting))
		if waiting > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the statement %s never waited for the lock", marker)
}

// A download reads its members, then stores the file. A member deleted and
// committed in between must not leave a stored file that names it: the
// download fails as a changed file and stores nothing. A deletion that
// comes after the download stored its file waits for it and takes the file.
func TestSEPADebitFileDownloadRacesTheDeletionOfAMember(t *testing.T) {
	t.Parallel()
	store := NewStore()

	t.Run("member deleted before the file is stored", func(t *testing.T) {
		f := newFileScope(t, "Träger Datei Wettlauf A")
		read, deleted := make(chan struct{}), make(chan struct{})
		result := make(chan error, 1)
		go func() {
			result <- testpkg.WithinTraeger(f.ctx, f.organization, func(ctx context.Context) error {
				if _, err := store.SEPADebitsOfRun(ctx, f.organization, "care", f.run, true, false); err != nil {
					return err
				}
				close(read)
				<-deleted
				return store.SaveSEPADebitFile(ctx, f.organization, 0, f.file(fmt.Sprintf("MSG-A-%d", f.unique)))
			})
		}()
		<-read
		_, err := f.db.ExecContext(context.Background(), `DELETE FROM parent_billing.sepa_debits WHERE id = ?`, f.second)
		require.NoError(t, err)
		close(deleted)
		err = <-result
		require.Error(t, err, "the download of a file whose member went is refused")
		assert.ErrorIs(t, err, parentbilling.ErrSEPADebitsChanged)
		assert.Empty(t, f.kept(t), "no stored file names a deleted member")
	})

	t.Run("member deleted after the file is stored", func(t *testing.T) {
		f := newFileScope(t, "Träger Datei Wettlauf B")
		stored, commit := make(chan struct{}), make(chan struct{})
		release := sync.OnceFunc(func() { close(commit) })
		// A failed assertion still ends the download's transaction.
		t.Cleanup(release)
		result := make(chan error, 1)
		go func() {
			result <- testpkg.WithinTraeger(f.ctx, f.organization, func(ctx context.Context) error {
				if err := store.SaveSEPADebitFile(ctx, f.organization, 0, f.file(fmt.Sprintf("MSG-B-%d", f.unique))); err != nil {
					close(stored)
					return err
				}
				close(stored)
				<-commit
				return nil
			})
		}()
		<-stored
		marker := fmt.Sprintf("file-race-%d", f.unique)
		deletion := make(chan error, 1)
		go func() {
			_, err := f.db.ExecContext(context.Background(), `/* `+marker+` */ DELETE FROM parent_billing.sepa_debits WHERE id = ?`, f.second)
			deletion <- err
		}()
		waitForLock(t, f.db, marker, deletion)
		release()
		require.NoError(t, <-result)
		require.NoError(t, <-deletion)
		assert.Empty(t, f.kept(t), "the deletion waited for the download and took its file")
	})
}

// A submitted file is evidence of what the bank received: it stays while
// one of its debits stays and goes with the last (ADR 0187). A mandate of
// the file does not take it while its debits stay. An unsubmitted download
// goes with any member.
func TestSubmittedSEPADebitFileGoesWithItsLastDebit(t *testing.T) {
	t.Parallel()
	f := newFileScope(t, "Träger Datei Original")
	ctx := context.Background()
	original, download := fmt.Sprintf("MSG-O-%d", f.unique), fmt.Sprintf("MSG-U-%d", f.unique)
	f.keep(t, original, true, 0, []int64{f.debit, f.second}, []int64{f.mandate, f.otherMandate})
	f.keep(t, download, false, 0, []int64{f.debit, f.second}, []int64{f.mandate})

	_, err := f.db.ExecContext(ctx, `DELETE FROM parent_billing.sepa_mandates WHERE id = ?`, f.otherMandate)
	require.NoError(t, err)
	assert.Equal(t, []string{original, download}, f.kept(t), "a mandate does not take a submitted file whose debits stay")

	_, err = f.db.ExecContext(ctx, `DELETE FROM parent_billing.sepa_debits WHERE id = ?`, f.debit)
	require.NoError(t, err)
	assert.Equal(t, []string{original}, f.kept(t), "the download goes with its first member, the original stays with the other debit")

	_, err = f.db.ExecContext(ctx, `DELETE FROM parent_billing.sepa_debits WHERE id = ?`, f.second)
	require.NoError(t, err)
	assert.Empty(t, f.kept(t), "the original goes with its last debit")
}

// The last two debits of a submitted file deleted at once in two
// transactions: neither may keep the file because the other's debit still
// looked present.
func TestSubmittedSEPADebitFileGoesWhenItsLastDebitsGoConcurrently(t *testing.T) {
	t.Parallel()
	f := newFileScope(t, "Träger Datei Original parallel")
	ctx := context.Background()
	f.keep(t, fmt.Sprintf("MSG-P-%d", f.unique), true, 0, []int64{f.debit, f.second}, []int64{f.mandate})

	first, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = first.Rollback() }()
	_, err = first.ExecContext(ctx, `DELETE FROM parent_billing.sepa_debits WHERE id = ?`, f.debit)
	require.NoError(t, err)

	marker := fmt.Sprintf("file-last-%d", f.unique)
	deletion := make(chan error, 1)
	go func() {
		_, err := f.db.ExecContext(context.Background(), `/* `+marker+` */ DELETE FROM parent_billing.sepa_debits WHERE id = ?`, f.second)
		deletion <- err
	}()
	waitForLock(t, f.db, marker, deletion)
	require.NoError(t, first.Commit())
	require.NoError(t, <-deletion)
	assert.Empty(t, f.kept(t), "the original goes with its last debit, whichever transaction deletes it")
}

// The request role may store and submit files but delete none (ADR 0187):
// the cleanups run through owner functions scoped to the Träger of the
// transaction. A member deleted by the request role still takes its file.
func TestRequestRoleCannotDeleteASEPADebitFile(t *testing.T) {
	t.Parallel()
	f := newFileScope(t, "Träger Datei Rechte")
	store := NewStore()
	original, download := fmt.Sprintf("MSG-R-%d", f.unique), fmt.Sprintf("MSG-S-%d", f.unique)
	f.keep(t, original, true, 0, []int64{f.debit, f.second}, []int64{f.mandate})
	f.keep(t, download, false, 0, []int64{f.second}, []int64{f.mandate, f.otherMandate})

	err := testpkg.WithinTraeger(f.ctx, f.organization, func(ctx context.Context) error {
		tx, err := traegerDB(ctx, f.organization)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `DELETE FROM parent_billing.sepa_debit_files WHERE organization_id = ? AND message_id = ?`,
			f.organization, original)
		return err
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
	assert.Equal(t, []string{original, download}, f.kept(t), "the request role deletes no stored file")

	// The supersede of a submission is an owner function scoped to the
	// Träger of the transaction.
	err = testpkg.WithinTraeger(f.ctx, f.organization, func(ctx context.Context) error {
		tx, err := traegerDB(ctx, f.organization)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `SELECT parent_billing.drop_superseded_sepa_debit_files(?, 'care', ?, 'X', ?::bigint[], '{}'::bigint[])`,
			f.organization+1, f.run, members([]int64{f.second}))
		return err
	})
	require.Error(t, err, "another Träger's downloads are out of reach")
	assert.Contains(t, err.Error(), "Träger")

	// A contract the request role deletes takes its mandate and the
	// download that holds the mandate's account.
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.organization, func(ctx context.Context) error {
		tx, err := traegerDB(ctx, f.organization)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `DELETE FROM care_contract.contracts WHERE organization_id = ? AND id = ?`, f.organization, f.otherContract)
		return err
	}))
	assert.Equal(t, []string{original}, f.kept(t), "the download went with its mandate")

	// Download and submission still work under the request role; the
	// submission drops the unsubmitted download it superseded.
	superseded, submitted := fmt.Sprintf("MSG-T-%d", f.unique), fmt.Sprintf("MSG-V-%d", f.unique)
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.organization, func(ctx context.Context) error {
		if err := store.SaveSEPADebitFile(ctx, f.organization, 0, f.file(superseded)); err != nil {
			return err
		}
		return store.SubmitSEPADebitFile(ctx, f.organization, 0, f.file(submitted))
	}))
	assert.Equal(t, []string{original, submitted}, f.kept(t), "the submission dropped the download it superseded")
}

// An unsubmitted download is evidence of nothing: it expires 30 days after
// the download (ADR 0187). A submitted original stays, and only the hourly
// administrative cleanup may run the expiry.
func TestUnsubmittedSEPADebitDownloadsExpire(t *testing.T) {
	t.Parallel()
	f := newFileScope(t, "Träger Datei Ablauf")
	store := NewStore()
	expired, fresh, original := fmt.Sprintf("MSG-E-%d", f.unique), fmt.Sprintf("MSG-F-%d", f.unique), fmt.Sprintf("MSG-G-%d", f.unique)
	f.keep(t, expired, false, 31*24*time.Hour, []int64{f.debit}, []int64{f.mandate})
	f.keep(t, fresh, false, 29*24*time.Hour, []int64{f.second}, []int64{f.mandate})
	f.keep(t, original, true, 400*24*time.Hour, []int64{f.debit, f.second}, []int64{f.mandate})

	err := testpkg.WithinTraeger(f.ctx, f.organization, func(ctx context.Context) error {
		_, err := store.DeleteExpiredSEPADebitDownloads(ctx)
		return err
	})
	require.Error(t, err, "the expiry runs only in the administrative transaction")
	err = testpkg.WithinTraeger(f.ctx, f.organization, func(ctx context.Context) error {
		tx, err := traegerDB(ctx, f.organization)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `SELECT parent_billing.delete_expired_sepa_debit_downloads()`)
		return err
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied", "the request role may not run the expiry")

	var deleted int
	require.NoError(t, testpkg.WithinAdminTransaction(f.ctx, func(ctx context.Context) error {
		deleted, err = store.DeleteExpiredSEPADebitDownloads(ctx)
		return err
	}))
	assert.GreaterOrEqual(t, deleted, 1)
	assert.Equal(t, []string{fresh, original}, f.kept(t), "only the download older than 30 days expired")
}
