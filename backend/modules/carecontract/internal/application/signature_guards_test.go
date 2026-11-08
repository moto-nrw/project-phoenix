package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The guards of the signature (#4679) over the real store: the Träger
// filter of the paper contract beneath RLS, the single use of a link, and
// the races of a paper signature against the acceptance (#4680) and against
// the discard of the draft, in two real Träger transactions.

type guardSender struct{}

func (guardSender) TraegerLetterhead(context.Context, int64) (string, error) {
	return "Ganztag im Quartier e.V.\nAm Markt 4", nil
}

func (guardSender) WithdrawalReach(context.Context, int64) (carecontract.WithdrawalReach, error) {
	return carecontract.WithdrawalReach{}, nil
}

// signatures binds the paper signature of the fixture's contracts.
func (f raceFixture) signatures() *Signatures {
	return NewSignatures(postgres.NewStore(), SignaturePorts{
		Ledger: struct{ carecontract.SignatureLedger }{}, Links: f.contracts.links, Sender: guardSender{},
		Post: &racePost{next: time.Now().UnixNano()}, Runtime: raceRuntime{},
	}, Clock{Now: time.Now}, f.contracts)
}

func paperOf(name string) carecontract.PaperInput {
	return carecontract.PaperInput{SignerName: name, Filename: "vertrag.pdf", PDF: []byte("%PDF-1.4 unterschrieben"),
		SignedOn: calendar.NewDate(2026, 11, 2)}
}

func (f raceFixture) signedPapers(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_signatures WHERE contract_id = ?`, f.contract).
		Scan(context.Background(), &n))
	return n
}

// TestPaperSignatureOfAnotherTraegerFindsNothingBeneathRLS pins the Träger
// filter of the paper contract in the application and the store, beneath
// RLS: Träger B records a paper contract for a draft of Träger A and finds
// nothing; A's draft keeps no PDF and no signature. It goes red when the
// store reads the contract without its organization.
func TestPaperSignatureOfAnotherTraegerFindsNothingBeneathRLS(t *testing.T) {
	t.Parallel()
	f := newRaceFixture(t, "Träger Papier A")
	other := testpkg.CreateTestOrganization(t, f.db, &testpkg.Organization{
		Name: "Träger Papier B", Slug: fmt.Sprintf("traeger-papier-b-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerCareContracts(t, f.db, other)
	foreign := carecontract.Reviewer{OrganizationID: other, AccountID: f.reviewer.AccountID}

	err := testpkg.WithinTraeger(f.ctx, other, func(txCtx context.Context) error {
		// As the owner of the tables, beneath RLS: only the store's own
		// Träger filter keeps A's contract out.
		require.NoError(t, testpkg.WithoutRequestRole(txCtx))
		return f.signatures().RecordPaperSignature(txCtx, foreign, f.contract, paperOf("Fremde Person"))
	})
	require.ErrorIs(t, err, carecontract.ErrContractNotFound, "another Träger's contract is not found")
	s := f.state(t)
	assert.Equal(t, carecontract.StatusDraft, s.status, "A's draft stays a draft")
	assert.Zero(t, s.documents, "no PDF frozen for another Träger")
	assert.Zero(t, f.signedPapers(t), "no signature for another Träger")
}

// TestSigningLinkClosesOnce pins the single use of a link in the store: a
// link once used can never be used again, even while its contract could
// still be signed. It goes red when the update closes a used link again.
func TestSigningLinkClosesOnce(t *testing.T) {
	t.Parallel()
	f := newRaceFixture(t, "Träger Link einmal")
	store := postgres.NewStore()
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, func(txCtx context.Context) error {
		_, err := f.contracts.SendContracts(txCtx, f.reviewer, []int64{f.contract})
		return err
	}))
	err := testpkg.WithinTraeger(f.ctx, f.org, func(txCtx context.Context) error {
		link, found, err := store.LatestSigningLink(txCtx, f.org, f.contract)
		if err != nil || !found {
			return fmt.Errorf("the send made no link: %w", err)
		}
		now := time.Now()
		if err := store.UseSigningLink(txCtx, f.org, link.ID, now); err != nil {
			return fmt.Errorf("first use: %w", err)
		}
		return store.UseSigningLink(txCtx, f.org, link.ID, now.Add(time.Second))
	})
	require.ErrorIs(t, err, carecontract.ErrSigningLinkInvalid, "the second use of the same link fails")
}

// paperLoss reports an error a losing paper signature may end with.
func paperLoss(err error) bool {
	return errors.Is(err, carecontract.ErrContractNotSignable) || errors.Is(err, carecontract.ErrContractNotFound)
}

// TestPaperSignatureAgainstAcceptanceLetsOneWin is the paper contract of a
// draft the conclusion at the acceptance concludes at the same time. The
// acceptance holds the contract; the paper signature waits on its lock and
// then finds a concluded contract: one conclusion, mit_anmeldung, no
// signature, chk_contracts_active_concluded kept. It goes red when the
// paper signature reads the contract without its row lock.
func TestPaperSignatureAgainstAcceptanceLetsOneWin(t *testing.T) {
	t.Parallel()
	f := newRaceFixture(t, "Träger Rennen Papier Annahme")
	paper := func(txCtx context.Context) error {
		return f.signatures().RecordPaperSignature(txCtx, f.reviewer, f.contract, paperOf("Erika Muster"))
	}
	won, lost := f.race(t, f.conclude, paper)

	require.NoError(t, won, "the acceptance wins")
	require.Error(t, lost)
	assert.True(t, paperLoss(lost), "the paper signature gets a defined error, got %v", lost)
	s := f.state(t)
	assert.Equal(t, raceState{found: true, status: carecontract.StatusActive, concluded: true, effective: 1, versions: 1,
		documents: 1, mailings: 1, conclusions: 1}, s)
	assert.Zero(t, f.signedPapers(t), "the acceptance concluded it; no paper signature")
	var method string
	require.NoError(t, f.db.NewRaw(`SELECT conclusion_method FROM care_contract.contracts WHERE id = ?`, f.contract).
		Scan(context.Background(), &method))
	assert.Equal(t, carecontract.MethodWithApplication, method)
}

// TestPaperSignatureAgainstDiscardLetsOneWin is the paper contract of a
// draft the Träger-Büro discards at the same time. The discard holds the
// contract; the paper signature waits on its lock and then finds nothing:
// no PDF, signature or conclusion of a discarded draft. It goes red when
// the paper signature reads the contract without its row lock.
func TestPaperSignatureAgainstDiscardLetsOneWin(t *testing.T) {
	t.Parallel()
	f := newRaceFixture(t, "Träger Rennen Papier Verwerfen")
	discard := func(txCtx context.Context) error { return f.contracts.DiscardContract(txCtx, f.reviewer, f.contract) }
	paper := func(txCtx context.Context) error {
		return f.signatures().RecordPaperSignature(txCtx, f.reviewer, f.contract, paperOf("Erika Muster"))
	}
	won, lost := f.race(t, discard, paper)

	require.NoError(t, won, "the discard wins")
	require.Error(t, lost)
	assert.True(t, paperLoss(lost), "the paper signature gets a defined error, got %v", lost)
	s := f.state(t)
	assert.False(t, s.found, "the draft is gone")
	assert.Zero(t, s.documents+s.mailings+s.conclusions, "no PDF, mail or conclusion of a discarded draft")
	assert.Zero(t, f.signedPapers(t), "no signature of a discarded draft")
	assert.Zero(t, s.broken)
}

// TestSignatureStaysVisibleAfterAnApprovedChange pins that an approved
// change (#4681) never hides the proof of the signature: after a newer
// version follows the signed one, the detail still shows the signature,
// checked against the version and PDF it signed. It goes red when the
// detail looks the signature up by the newest version.
func TestSignatureStaysVisibleAfterAnApprovedChange(t *testing.T) {
	t.Parallel()
	f := newRaceFixture(t, "Träger Unterschrift nach Änderung")
	store := postgres.NewStore()
	// The contract began on 01.01.2026, before anything was signed.
	_, err := f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_versions SET effective_from = '2026-01-01'
		WHERE contract_id = ?`, f.contract)
	require.NoError(t, err)
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, func(txCtx context.Context) error {
		return f.signatures().RecordPaperSignature(txCtx, f.reviewer, f.contract, paperOf("Erika Muster"))
	}))
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, func(txCtx context.Context) error {
		record, err := store.Contract(txCtx, f.org, f.contract, true)
		if err != nil {
			return err
		}
		next := record.Version
		next.Number, next.Status, next.Source = 2, carecontract.VersionEffective, carecontract.SourceBookingChange
		next.EffectiveFrom = calendar.NewDate(2027, 9, 1)
		_, err = store.InsertVersion(txCtx, f.org, f.contract,
			VersionStep{Supersede: record.Version.ID, Status: carecontract.VersionEffective}, next)
		return err
	}))

	// The change began on 01.02.2026, so it is the version in force today.
	// The guard of 1.15.787 refuses a new version that starts in the past,
	// so the test moves its day as owner of the table; the signed first
	// version keeps every value it was signed with.
	_, err = f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_versions SET effective_from = '2026-02-01'
		WHERE contract_id = ? AND version_number = 2`, f.contract)
	require.NoError(t, err)

	detail, err := f.contracts.Contract(f.ctx, f.reviewer, f.contract)
	require.NoError(t, err)
	require.Len(t, detail.Versions, 2, "the approved change made a second version")
	assert.Equal(t, 2, detail.Version.Number, "the change is the version in force")
	require.NotNil(t, detail.Signature, "the signature stays visible after the change")
	assert.Equal(t, carecontract.SignaturePaper, detail.Signature.Method)
	assert.True(t, detail.Signature.Intact, "checked against the version and PDF it signed")
}
