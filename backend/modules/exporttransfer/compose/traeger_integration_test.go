package compose

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/exporttransfer"
	"github.com/moto-ogs/project-phoenix/modules/exporttransfer/internal/adapters/sftp"
	"github.com/moto-ogs/project-phoenix/modules/exporttransfer/internal/adapters/sftp/sftptest"
	"github.com/moto-ogs/project-phoenix/modules/exporttransfer/internal/domain"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The Träger scope of Export Transfer (ADR 0218) against PostgreSQL and a
// real SFTP counterpart. Each test names the protection it proves; removing
// that protection turns it red.

// contextCipher stands in for the bank cipher: it binds a value to its
// context like AES-GCM's associated data and never stores the plain bytes.
// The real cipher and the password context are proven together in
// cmd/bank_data_rekey_test.go.
type contextCipher struct{}

func (contextCipher) BankDataRing() {}

func (contextCipher) Seal(plaintext, context []byte) ([]byte, error) {
	sealed := append([]byte{byte(len(context))}, context...)
	for _, b := range plaintext {
		sealed = append(sealed, b^0x5a)
	}
	return sealed, nil
}

func (contextCipher) Open(sealed, context []byte) ([]byte, error) {
	if len(sealed) < 1+len(context) || int(sealed[0]) != len(context) || !bytes.Equal(sealed[1:1+len(context)], context) {
		return nil, errors.New("value does not open in this context")
	}
	plain := make([]byte, 0, len(sealed)-1-len(context))
	for _, b := range sealed[1+len(context):] {
		plain = append(plain, b^0x5a)
	}
	return plain, nil
}

type traegerFixture struct {
	db        *bun.DB
	ctx       context.Context
	transfers exporttransfer.TraegerTransfers
	server    sftptest.Server
	dir       string
	org       int64
	other     int64
}

func newTraegerFixture(t *testing.T, policy sftp.AddressPolicy) traegerFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.NewDatabase(t, db).UnitOfWork())
	newOrg := func(name string) int64 {
		return testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
			Name: name, Slug: fmt.Sprintf("export-%s-%d", name, testpkg.UniqueTestTenantID(t)),
		}).ID
	}
	if policy == nil {
		policy = sftptest.AllowLoopbackPolicy{}
	}
	transfers, err := NewTraegerTransfers(contextCipher{}, nil, sftp.WithAddressPolicy(policy))
	require.NoError(t, err)
	return traegerFixture{db: db, ctx: ctx, transfers: transfers, server: sftptest.Start(t), dir: t.TempDir(),
		org: newOrg("traeger"), other: newOrg("other")}
}

func (f traegerFixture) readyInput() exporttransfer.TraegerRecipientInput {
	password := f.server.Password
	return exporttransfer.TraegerRecipientInput{Name: "Kämmerei", Enabled: true, Host: f.server.Host, Port: f.server.Port,
		Username: sftptest.User, Password: &password, RemoteDirectory: f.dir, HostKeyFingerprint: f.server.Fingerprint}
}

// add enters a recipient of org and answers its id.
func (f traegerFixture) add(t *testing.T, org int64, input exporttransfer.TraegerRecipientInput) int64 {
	t.Helper()
	recipient, err := f.transfers.CreateRecipient(f.ctx, org, 0, input)
	require.NoError(t, err)
	require.Empty(t, recipient.Formats, "a new recipient receives no format until the Träger assigns one")
	return recipient.ID
}

// assign sends each format to its recipient, 0 for the download.
func (f traegerFixture) assign(t *testing.T, org int64, recipients map[string]int64) exporttransfer.TraegerRecipientsPage {
	t.Helper()
	assignments := make([]exporttransfer.TraegerAssignment, 0, len(recipients))
	for format, id := range recipients {
		assignment := exporttransfer.TraegerAssignment{Format: format}
		if id > 0 {
			assignment.RecipientID = &id
		}
		assignments = append(assignments, assignment)
	}
	page, err := f.transfers.AssignFormats(f.ctx, org, 0, assignments)
	require.NoError(t, err)
	return page
}

// save enters the ready recipient of org that receives every format.
func (f traegerFixture) save(t *testing.T, org int64) int64 {
	t.Helper()
	id := f.add(t, org, f.readyInput())
	f.assign(t, org, map[string]int64{"sollstellung": id, "datev": id, "pain008": id})
	page, err := f.transfers.Recipient(f.ctx, org, id, 10)
	require.NoError(t, err)
	require.True(t, page.Recipient.Ready)
	require.Equal(t, exporttransfer.TraegerFormats, page.Recipient.Formats)
	return id
}

func financeRequest(org int64, filename string, settled *[]exporttransfer.Outcome) exporttransfer.TraegerTransferRequest {
	return exporttransfer.TraegerTransferRequest{OrganizationID: org, Kind: exporttransfer.KindParentBillingFinance, Format: "sollstellung",
		PrepareFile: func(context.Context, exporttransfer.Destination) (string, []byte, error) {
			return filename, []byte("Debitor;Betrag\r\n10001;78,00\r\n"), nil
		},
		Settle: func(ctx context.Context, outcome exporttransfer.Outcome) error {
			if tenant.TraegerTransactionFromContext(ctx) != org {
				return errors.New("settlement outside the Träger transaction")
			}
			*settled = append(*settled, outcome)
			return nil
		},
	}
}

// journalRows reads the Träger journal as the owner of the table, without
// row level security.
func journalRows(t *testing.T, ctx context.Context, db *bun.DB, org int64) []struct {
	Status        string `bun:"status"`
	FailureReason string `bun:"failure_reason"`
	DispatchState string `bun:"dispatch_state"`
	TargetHost    string `bun:"target_host"`
} {
	t.Helper()
	var rows []struct {
		Status        string `bun:"status"`
		FailureReason string `bun:"failure_reason"`
		DispatchState string `bun:"dispatch_state"`
		TargetHost    string `bun:"target_host"`
	}
	require.NoError(t, db.NewRaw(`SELECT status, failure_reason, dispatch_state, target_host FROM audit.traeger_export_transfers
		WHERE organization_id = ? ORDER BY id`, org).Scan(ctx, &rows))
	return rows
}

func TestTraegerHandoverDeliversAndSettlesJournalAndProducerTogether(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	f.save(t, f.org)
	var settled []exporttransfer.Outcome
	outcome, err := f.transfers.Transfer(f.ctx, financeRequest(f.org, "sollstellung-110.csv", &settled))
	require.NoError(t, err)
	require.True(t, outcome.Transferred)
	arrived, err := os.ReadFile(filepath.Join(f.dir, "sollstellung-110.csv"))
	require.NoError(t, err)
	assert.Equal(t, "Debitor;Betrag\r\n10001;78,00\r\n", string(arrived))
	require.Len(t, settled, 1, "the producer settles its protocol in the journal's completing transaction")
	assert.True(t, settled[0].Transferred)
	rows := journalRows(t, f.ctx, f.db, f.org)
	require.Len(t, rows, 1)
	assert.Equal(t, "success", rows[0].Status)
	assert.Equal(t, "terminal", rows[0].DispatchState)
	assert.Empty(t, journalRows(t, f.ctx, f.db, f.other), "another Träger's journal stays empty")
	var schoolRows int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM audit.export_transfers WHERE filename = 'sollstellung-110.csv'`).Scan(f.ctx, &schoolRows))
	assert.Zero(t, schoolRows, "the Träger scope never writes the school journal")
}

func TestTraegerHandoverWithoutReadyRecipientPreparesNothing(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	prepared := false
	request := exporttransfer.TraegerTransferRequest{OrganizationID: f.org, Kind: exporttransfer.KindParentBillingFinance, Format: "datev",
		PrepareFile: func(context.Context, exporttransfer.Destination) (string, []byte, error) {
			prepared = true
			return "x.csv", []byte("x"), nil
		}}
	outcome, err := f.transfers.Transfer(f.ctx, request)
	require.NoError(t, err)
	assert.Equal(t, exporttransfer.ReasonNotConfigured, outcome.Reason)
	// A switched-off recipient of the format, a ready recipient of this
	// Träger that receives other formats only and a ready one of another
	// Träger change nothing.
	input := f.readyInput()
	input.Enabled = false
	off := f.add(t, f.org, input)
	f.assign(t, f.org, map[string]int64{"datev": off})
	outcome, err = f.transfers.Transfer(f.ctx, request)
	require.NoError(t, err)
	assert.Equal(t, exporttransfer.ReasonNotConfigured, outcome.Reason)
	input = f.readyInput()
	input.Name = "Steuerbüro"
	ready := f.add(t, f.org, input)
	f.assign(t, f.org, map[string]int64{"sollstellung": ready, "pain008": ready})
	f.save(t, f.other)
	outcome, err = f.transfers.Transfer(f.ctx, request)
	require.NoError(t, err)
	assert.Equal(t, exporttransfer.ReasonNotConfigured, outcome.Reason)
	assert.False(t, prepared, "no file and no file number without a ready recipient of this format of this Träger")
	assert.Empty(t, journalRows(t, f.ctx, f.db, f.org))
}

func TestTraegerHandoverCommitsFileAndIntentBeforeConnecting(t *testing.T) {
	t.Parallel()
	var f traegerFixture
	connects := 0
	f = newTraegerFixture(t, inspectingPolicy{beforeConnect: func() {
		connects++
		rows := journalRows(t, f.ctx, f.db, f.org)
		require.Len(t, rows, 1, "the intent commits before any connection")
		assert.Equal(t, "dispatching", rows[0].DispatchState)
	}})
	f.save(t, f.org)
	var settled []exporttransfer.Outcome
	outcome, err := f.transfers.Transfer(f.ctx, financeRequest(f.org, "datev-111.csv", &settled))
	require.NoError(t, err)
	assert.True(t, outcome.Transferred)
	assert.Equal(t, 1, connects)
}

func TestTraegerHandoverFailureIsAResultInJournalAndProtocol(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	input := f.readyInput()
	input.HostKeyFingerprint = f.server.FingerprintECDSA
	id := f.add(t, f.org, input)
	f.assign(t, f.org, map[string]int64{"sollstellung": id})
	var settled []exporttransfer.Outcome
	outcome, err := f.transfers.Transfer(f.ctx, financeRequest(f.org, "pain-112.xml", &settled))
	require.NoError(t, err)
	assert.False(t, outcome.Transferred)
	assert.Equal(t, exporttransfer.ReasonHostKey, outcome.Reason)
	require.Len(t, settled, 1)
	assert.Equal(t, exporttransfer.ReasonHostKey, settled[0].Reason)
	rows := journalRows(t, f.ctx, f.db, f.org)
	require.Len(t, rows, 1)
	assert.Equal(t, "failed", rows[0].Status)
	assert.Equal(t, "host_key_mismatch", rows[0].FailureReason)
	_, err = os.Stat(filepath.Join(f.dir, "pain-112.xml"))
	assert.True(t, os.IsNotExist(err), "nothing lands at a counterpart that is not the pinned one")
	page, err := f.transfers.Recipients(f.ctx, f.org, 10)
	require.NoError(t, err)
	require.Len(t, page.Handovers, 1)
	assert.Equal(t, exporttransfer.TraegerFailed, page.Handovers[0].State)
	require.NotNil(t, page.Recipients[0].LastHandover, "the failure stays on its recipient until the next handover")
	assert.Equal(t, exporttransfer.TraegerFailed, page.Recipients[0].LastHandover.State)
}

func TestTraegerRecipientPasswordIsSealedBoundAndNeverAnswered(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	f.save(t, f.org)
	var sealed []byte
	require.NoError(t, f.db.NewRaw(`SELECT password_sealed FROM config.traeger_export_recipients WHERE organization_id = ?`, f.org).
		Scan(f.ctx, &sealed))
	assert.NotEmpty(t, sealed)
	assert.NotContains(t, string(sealed), f.server.Password, "the password is stored sealed only")
	page, err := f.transfers.Recipients(f.ctx, f.org, 10)
	require.NoError(t, err)
	assert.True(t, page.Recipients[0].PasswordSet)
	answer, err := json.Marshal(page)
	require.NoError(t, err)
	assert.NotContains(t, string(answer), f.server.Password, "no answer carries the password")
	assert.NotContains(t, string(answer), "password\":", "no answer has a password field, not even a masked one")
	// A ciphertext copied to another Träger does not open there: its
	// context names the Träger it was sealed for.
	f.save(t, f.other)
	_, err = f.db.NewRaw(`UPDATE config.traeger_export_recipients SET password_sealed = ? WHERE organization_id = ?`, sealed, f.other).Exec(f.ctx)
	require.NoError(t, err)
	other, err := f.transfers.Recipients(f.ctx, f.other, 10)
	require.NoError(t, err)
	assert.False(t, other.Recipients[0].PasswordSet)
	assert.False(t, other.Recipients[0].Ready)
	assert.Contains(t, other.Recipients[0].MissingFields, exporttransfer.RecipientFieldPassword)
}

func TestTraegerRecipientPasswordNeverFollowsAChangedDestination(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	id := f.save(t, f.org)
	for name, change := range map[string]func(*exporttransfer.TraegerRecipientInput){
		"host":        func(in *exporttransfer.TraegerRecipientInput) { in.Host = "attacker.example.org" },
		"port":        func(in *exporttransfer.TraegerRecipientInput) { in.Port = 2222 },
		"user":        func(in *exporttransfer.TraegerRecipientInput) { in.Username = "someone" },
		"fingerprint": func(in *exporttransfer.TraegerRecipientInput) { in.HostKeyFingerprint = f.server.FingerprintECDSA },
	} {
		input := f.readyInput()
		input.Password = nil
		change(&input)
		_, err := f.transfers.SaveRecipient(f.ctx, f.org, 0, id, input)
		var invalid *exporttransfer.RecipientError
		require.ErrorAs(t, err, &invalid, name)
		assert.Equal(t, []string{exporttransfer.RecipientFieldPassword}, invalid.Fields, name)
	}
	// The same destination keeps the stored password without entering it.
	input := f.readyInput()
	input.Password = nil
	input.RemoteDirectory = f.dir + "/"
	recipient, err := f.transfers.SaveRecipient(f.ctx, f.org, 0, id, input)
	require.NoError(t, err)
	assert.True(t, recipient.PasswordSet)
	page, err := f.transfers.Recipient(f.ctx, f.org, id, 10)
	require.NoError(t, err)
	assert.Equal(t, f.server.Host, page.Recipient.Host)
	// A second recipient of the same Träger never takes the stored password
	// of the first: it starts without one.
	second := f.readyInput()
	second.Name, second.Password = "Steuerbüro", nil
	created, err := f.transfers.CreateRecipient(f.ctx, f.org, 0, second)
	require.NoError(t, err)
	assert.False(t, created.PasswordSet)
	assert.Contains(t, created.MissingFields, exporttransfer.RecipientFieldPassword)
}

func TestTraegerRecipientRemovalTakesThePassword(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	id := f.save(t, f.org)
	otherID := f.save(t, f.other)
	_, err := f.transfers.DeleteRecipient(f.ctx, f.org, otherID)
	require.ErrorIs(t, err, exporttransfer.ErrRecipientNotFound, "a Träger cannot remove another Träger's recipient")
	page, err := f.transfers.DeleteRecipient(f.ctx, f.org, id)
	require.NoError(t, err)
	assert.Empty(t, page.Recipients)
	var left int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM config.traeger_export_recipients WHERE organization_id = ?`, f.org).Scan(f.ctx, &left))
	assert.Zero(t, left)
	other, err := f.transfers.Recipient(f.ctx, f.other, otherID, 10)
	require.NoError(t, err)
	assert.True(t, other.Recipient.Ready, "removing one Träger's recipient leaves another's")
}

// The store filters by the Träger itself: with row level security off, a
// Träger still reads, changes and lists only its own rows (brief rule 25).
func TestTraegerStoreFiltersTheTraegerBelowRowLevelSecurity(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	// The other Träger enters its recipient first: an unfiltered read would
	// meet its rows before the own ones.
	foreign := f.save(t, f.other)
	own := f.save(t, f.org)
	var settled []exporttransfer.Outcome
	_, err := f.transfers.Transfer(f.ctx, financeRequest(f.other, "other-1.csv", &settled))
	require.NoError(t, err)
	store := traegerStore()
	err = tenant.WithinTraeger(f.ctx, f.org, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		rows, err := store.List(ctx, f.org, 0, 50)
		require.NoError(t, err)
		assert.Empty(t, rows, "the journal of another Träger stays hidden without RLS")
		rows, err = store.List(ctx, f.org, foreign, 50)
		require.NoError(t, err)
		assert.Empty(t, rows, "naming another Träger's recipient reads none of its journal")
		recipients, err := store.Recipients(ctx, f.org)
		require.NoError(t, err)
		require.Len(t, recipients, 1, "the list holds only the own recipient")
		assert.Equal(t, own, recipients[0].ID)
		assert.Nil(t, recipients[0].LastHandover, "another Träger's handover is never the own recipient's last one")
		_, found, err := store.Recipient(ctx, f.org, foreign)
		require.NoError(t, err)
		assert.False(t, found, "another Träger's recipient is not found by its id")
		_, found, err = store.LockRecipient(ctx, f.org, foreign)
		require.NoError(t, err)
		assert.False(t, found, "the lock reaches only the own recipient")
		recipient, found, err := store.RecipientForFormat(ctx, f.org, "sollstellung")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, own, recipient.ID, "a format resolves to the own recipient only")
		locked, err := store.LockRecipientsAfter(ctx, f.org, 0, 10)
		require.NoError(t, err)
		require.Len(t, locked, 1, "the rekey locks only the own recipients")
		updated, err := store.UpdateRecipient(ctx, domain.StoredRecipient{ID: foreign, OrganizationID: f.org, Name: "Fremd", Port: 22}, 0)
		require.NoError(t, err)
		assert.False(t, updated, "an update with another Träger's id changes nothing")
		// Both Träger name their recipient "Kämmerei": the name is taken only
		// within one Träger.
		updated, err = store.UpdateRecipient(ctx, recipients[0], 0)
		require.NoError(t, err, "another Träger's recipient of the same name never blocks a save")
		assert.True(t, updated)
		assigned, err := store.AssignFormat(ctx, f.org, "datev", foreign, 0)
		require.NoError(t, err)
		assert.False(t, assigned, "a format never goes to another Träger's recipient")
		require.NoError(t, store.ReplaceSealedPassword(ctx, f.org, foreign, []byte("rewrapped")))
		deleted, err := store.DeleteRecipient(ctx, f.org, foreign)
		require.NoError(t, err)
		assert.False(t, deleted)
		require.NoError(t, store.ReplaceSealedPassword(ctx, f.org, own, []byte("rewrapped")))
		_, err = store.DeleteRecipient(ctx, f.org, own)
		return err
	})
	require.NoError(t, err)
	page, err := f.transfers.Recipient(f.ctx, f.other, foreign, 10)
	require.NoError(t, err)
	assert.True(t, page.Recipient.Ready, "a delete, an update, an assignment and a rewrap without RLS reach only their own Träger")
	assert.Equal(t, "Kämmerei", page.Recipient.Name)
	assert.Equal(t, exporttransfer.TraegerFormats, page.Recipient.Formats)
	assert.Len(t, page.Handovers, 1)
}

// The store refuses a Träger other than the one of its transaction, and a
// school transaction never sees a Träger row.
func TestTraegerStoreRefusesAForeignTransaction(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	id := f.save(t, f.org)
	store := traegerStore()
	err := tenant.WithinTraeger(f.ctx, f.other, func(ctx context.Context) error {
		_, _, err := store.Recipient(ctx, f.org, id)
		return err
	})
	require.Error(t, err)
	err = tenant.WithinTraeger(f.ctx, f.other, func(ctx context.Context) error {
		var token string
		return requestTx(t, ctx).NewRaw(`SELECT audit.claim_traeger_export_transfer(?, 1)`, f.org).Scan(ctx, &token)
	})
	require.Error(t, err, "the fence functions check the Träger of the transaction")
	testpkg.OwnTenant(t)
	school := testpkg.Ctx(t)
	require.NoError(t, tenant.WithinCurrentTenant(school, func(ctx context.Context) error {
		transaction, _ := tenant.TransactionFromContext(ctx)
		var visible int
		require.NoError(t, transaction.(bun.Tx).NewRaw(`SELECT count(*) FROM config.traeger_export_recipients`).Scan(ctx, &visible))
		assert.Zero(t, visible, "a school transaction sees no recipient of a Träger")
		require.NoError(t, transaction.(bun.Tx).NewRaw(`SELECT count(*) FROM config.traeger_export_recipient_formats`).Scan(ctx, &visible))
		assert.Zero(t, visible, "a school transaction sees no assignment of a Träger")
		return nil
	}))
}

// Recovery settles an expired Träger intent as unknown and never dials.
func TestTraegerRecoverySettlesExpiredIntentsWithoutResending(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, inspectingPolicy{beforeConnect: func() { panic("crash before upload") }})
	f.save(t, f.org)
	var settled []exporttransfer.Outcome
	require.Panics(t, func() { _, _ = f.transfers.Transfer(f.ctx, financeRequest(f.org, "crash-1.csv", &settled)) })
	_, err := f.db.NewRaw(`UPDATE audit.traeger_export_transfers SET dispatch_expires_at = clock_timestamp() - INTERVAL '1 second'
		WHERE organization_id = ?`, f.org).Exec(f.ctx)
	require.NoError(t, err)
	n, err := NewDispatchRecovery()(f.ctx, 1000)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1)
	rows := journalRows(t, f.ctx, f.db, f.org)
	require.Len(t, rows, 1)
	assert.Equal(t, "unknown", rows[0].DispatchState)
	page, err := f.transfers.Recipients(f.ctx, f.org, 10)
	require.NoError(t, err)
	assert.Equal(t, exporttransfer.TraegerUnknown, page.Handovers[0].State)
	assert.Equal(t, exporttransfer.TraegerUnknown, page.Recipients[0].LastHandover.State)
	assert.Empty(t, settled)
}

func TestTraegerScopeRefusesASchoolTransaction(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	f.save(t, f.org)
	testpkg.OwnTenant(t)
	school := testpkg.Ctx(t)
	err := tenant.WithinCurrentTenant(school, func(ctx context.Context) error {
		_, err := f.transfers.Recipients(ctx, f.org, 10)
		return err
	})
	require.Error(t, err, "the Träger scope never runs inside a school transaction")
}
