package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The races of the conclusion (#4680): two real Träger transactions on the
// same contract in the same test database. Invariant: at most one
// transition per contract wins; the loser gets a defined error; the final
// state meets every constraint. Each race goes red when the conclusion
// (concludeRecord) reads the contract without its row lock.

type raceRuntime struct{}

func (raceRuntime) WithinSchool(ctx context.Context, _ int64, fn func(context.Context) error) error {
	return fn(ctx)
}

func (raceRuntime) WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	return testpkg.WithinTraeger(ctx, organizationID, fn)
}

type raceLetterhead struct{}

func (raceLetterhead) Letterhead(context.Context, carecontract.Reviewer) (carecontract.Letterhead, error) {
	return carecontract.Letterhead{SenderText: "Ganztag im Quartier e.V.\nAm Markt 4", ContactName: "Paula Kern"}, nil
}

// raceRenderer lays out a fixed PDF; the races are about the rows, not the
// layout.
type raceRenderer struct{}

func (raceRenderer) RenderContract(_ Letter, filenameBase string) (carecontract.File, error) {
	return carecontract.File{Filename: filenameBase + ".pdf", ContentType: "application/pdf", Data: []byte("%PDF-1.4 Vertrag")}, nil
}

type raceAccess struct{}

func (raceAccess) RecordAccess(context.Context, carecontract.SchoolAccess) error { return nil }

// racePost answers every mail it enqueued as still waiting, safe for two
// transactions at once.
type racePost struct {
	mu   sync.Mutex
	next int64
	ids  []int64
}

func (p *racePost) EnqueueContractMail(context.Context, carecontract.ContractMail) (int64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next++
	p.ids = append(p.ids, p.next)
	return p.next, nil
}

// ContractMailBlock holds no address back.
func (p *racePost) ContractMailBlock(context.Context, int64, string, string) (string, error) {
	return "", nil
}

func (p *racePost) ContractMailStates(context.Context, int64, int64) ([]carecontract.MailState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	states := make([]carecontract.MailState, len(p.ids))
	for i, id := range p.ids {
		states[i] = carecontract.MailState{OutboxID: id, State: carecontract.MailQueued}
	}
	return states, nil
}

type raceFixture struct {
	db          *bun.DB
	ctx         context.Context
	org         int64
	reviewer    carecontract.Reviewer
	contract    int64
	contracts   *Contracts
	conclusions *Conclusions
	conclusion  carecontract.Concluded
}

// newRaceFixture is one draft of a Träger with its party and services, and
// the conclusion and the contracts over the real store.
func newRaceFixture(t *testing.T, name string) raceFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	org := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: name, Slug: fmt.Sprintf("traeger-rennen-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerCareContracts(t, db, org)
	account := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("rennen-%d@test.local", testpkg.UniqueTestTenantID(t)), testpkg.NewTestPassword())
	store := postgres.NewStore()
	clock := Clock{Now: time.Now}
	contracts := NewContracts(store, raceRenderer{}, ContractPorts{
		Letterheads: raceLetterhead{}, Post: &racePost{next: testpkg.UniqueTestTenantID(t)}, Runtime: raceRuntime{}, Access: raceAccess{},
	}, clock)
	var id int64
	require.NoError(t, testpkg.WithinTraeger(ctx, org, func(txCtx context.Context) error {
		var err error
		id, err = store.InsertContract(txCtx, org, 0, NewContract{
			Key: ContractKey{SchoolID: testpkg.UniqueTestTenantID(t), RequestChildID: testpkg.UniqueTestTenantID(t)},
			Version: carecontract.Version{
				Number: 1, Status: carecontract.VersionRequested, EffectiveFrom: calendar.NewDate(2027, 8, 1),
				ChildFirstName: "Lina", ChildLastName: "Muster", SchoolName: "OGS Am Lindenhof",
				Services: []carecontract.Service{{Weekday: "mon", CareOfferingID: 7, ModuleName: "Ganztag", EndsAt: "16:00"}},
				Terms: carecontract.TemplateInput{TermKind: carecontract.TermOpenEnded, NoticeMonths: 1,
					NoticeTo: carecontract.NoticeMonthEnd, ContractPartner: "Ganztag im Quartier e.V."},
			},
			Party: NewParty{FirstName: "Erika", LastName: "Muster", Email: "erika@example.org"},
		})
		return err
	}))
	return raceFixture{
		db: db, ctx: ctx, org: org, reviewer: carecontract.Reviewer{OrganizationID: org, AccountID: account.ID},
		contract: id, contracts: contracts,
		conclusions: &Conclusions{intake: &Intake{store: store}, contracts: contracts, clock: clock},
		conclusion:  carecontract.Concluded{Method: carecontract.MethodWithApplication, At: time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)},
	}
}

func (f raceFixture) conclude(txCtx context.Context) error {
	return f.conclusions.concludeRecord(txCtx, f.org, f.contract, f.conclusion)
}

// race runs first in a Träger transaction and holds it open before its
// commit; then starts second in its own Träger transaction. It releases the
// first once the second waits on a lock of the contract tables or has
// finished, and returns both results.
func (f raceFixture) race(t *testing.T, first, second func(context.Context) error) (error, error) {
	t.Helper()
	held, release := make(chan struct{}), make(chan struct{})
	firstDone, secondDone := make(chan error, 1), make(chan error, 1)
	go func() {
		firstDone <- testpkg.WithinTraeger(f.ctx, f.org, func(txCtx context.Context) error {
			err := first(txCtx)
			close(held)
			<-release
			return err
		})
	}()
	select {
	case <-held:
	case <-time.After(30 * time.Second):
		t.Fatal("the first transaction never reached its commit")
	}
	go func() {
		secondDone <- testpkg.WithinTraeger(f.ctx, f.org, second)
	}()
	var secondErr error
	finished := false
	require.Eventually(t, func() bool {
		select {
		case secondErr = <-secondDone:
			finished = true
			return true
		default:
		}
		var waiting int
		err := f.db.NewRaw(`SELECT COUNT(*) FROM pg_stat_activity WHERE datname = current_database()
			AND wait_event_type = 'Lock' AND query LIKE '%care_contract.%'`).Scan(context.Background(), &waiting)
		return err == nil && waiting > 0
	}, 30*time.Second, 10*time.Millisecond, "the second transaction waits or finishes")
	close(release)
	firstErr := <-firstDone
	if !finished {
		secondErr = <-secondDone
	}
	return firstErr, secondErr
}

// state is the contract as it ended, with the rows of its versions,
// documents, mails and conclusions; found is false when it is gone.
type raceState struct {
	found       bool
	status      string
	concluded   bool
	effective   int
	versions    int
	documents   int
	mailings    int
	conclusions int
	broken      int
}

func (f raceFixture) state(t *testing.T) raceState {
	t.Helper()
	ctx := context.Background()
	var s raceState
	var rows []struct {
		Status    string `bun:"status"`
		Concluded bool   `bun:"concluded"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT status, concluded_at IS NOT NULL AS concluded FROM care_contract.contracts WHERE id = ?`,
		f.contract).Scan(ctx, &rows))
	if len(rows) == 1 {
		s.found, s.status, s.concluded = true, rows[0].Status, rows[0].Concluded
	}
	count := func(query string, target *int) {
		require.NoError(t, f.db.NewRaw(query, f.contract).Scan(ctx, target))
	}
	count(`SELECT COUNT(*) FROM care_contract.contract_versions WHERE contract_id = ?`, &s.versions)
	count(`SELECT COUNT(*) FROM care_contract.contract_versions WHERE contract_id = ? AND status = 'wirksam'`, &s.effective)
	count(`SELECT COUNT(*) FROM care_contract.contract_documents WHERE contract_id = ?`, &s.documents)
	count(`SELECT COUNT(*) FROM care_contract.contract_mailings WHERE contract_id = ?`, &s.mailings)
	count(`SELECT COUNT(*) FROM care_contract.contract_events WHERE contract_id = ? AND action = 'conclude'`, &s.conclusions)
	// chk_contracts_active_concluded and its siblings, read back: no active
	// contract without its conclusion, no conclusion on another state.
	count(`SELECT COUNT(*) FROM care_contract.contracts WHERE id = ?
		AND ((status = 'aktiv') <> (concluded_at IS NOT NULL) OR (concluded_at IS NULL) <> (conclusion_method IS NULL))`, &s.broken)
	return s
}

// definedLoss reports an error a losing transition may end with; anything
// else (a unique or foreign key violation of a stale read) is no defined
// error.
func definedLoss(err error) bool {
	for _, defined := range []error{carecontract.ErrContractNotConcludable, carecontract.ErrContractNotFound,
		carecontract.ErrContractNotDraft, carecontract.ErrMailPending, carecontract.ErrContractNotSendable} {
		if errors.Is(err, defined) {
			return true
		}
	}
	return false
}

// TestConclusionAgainstConclusionLetsOneWin is the double acceptance: two
// conclusions of the same contract at once. Exactly one concluded contract
// with one effective version, one PDF and one mail; the second attempt gets
// a defined error.
func TestConclusionAgainstConclusionLetsOneWin(t *testing.T) {
	t.Parallel()
	f := newRaceFixture(t, "Träger Rennen Abschluss")
	won, lost := f.race(t, f.conclude, f.conclude)

	require.NoError(t, won, "the first conclusion wins")
	require.Error(t, lost)
	assert.True(t, definedLoss(lost), "the second conclusion gets a defined error, got %v", lost)
	s := f.state(t)
	assert.Equal(t, raceState{found: true, status: carecontract.StatusActive, concluded: true, effective: 1, versions: 1,
		documents: 1, mailings: 1, conclusions: 1}, s)
}

// TestConclusionAgainstDiscardLetsOneWin is the conclusion of a draft the
// Träger-Büro discards at the same time. Exactly one wins; the end state
// meets chk_contracts_active_concluded.
func TestConclusionAgainstDiscardLetsOneWin(t *testing.T) {
	t.Parallel()
	f := newRaceFixture(t, "Träger Rennen Verwerfen")
	discard := func(txCtx context.Context) error { return f.contracts.DiscardContract(txCtx, f.reviewer, f.contract) }
	won, lost := f.race(t, discard, f.conclude)

	require.NoError(t, won, "the discard wins")
	require.Error(t, lost)
	assert.True(t, definedLoss(lost), "the conclusion gets a defined error, got %v", lost)
	s := f.state(t)
	assert.False(t, s.found, "the draft is gone")
	assert.Zero(t, s.documents+s.mailings+s.conclusions, "no PDF, mail or conclusion of a discarded draft")
	assert.Zero(t, s.broken)
}

// TestConclusionAgainstSendLetsOneWin is the conclusion of a draft the
// Träger-Büro sends at the same time. Exactly one wins; no contract is sent
// and concluded at once, and one PDF and one mail exist.
func TestConclusionAgainstSendLetsOneWin(t *testing.T) {
	t.Parallel()
	f := newRaceFixture(t, "Träger Rennen Versand")
	send := func(txCtx context.Context) error {
		result, err := f.contracts.SendContracts(txCtx, f.reviewer, []int64{f.contract})
		if err == nil && len(result.Sent) != 1 {
			err = fmt.Errorf("the send was skipped: %v", result.Skipped)
		}
		return err
	}
	won, lost := f.race(t, send, f.conclude)

	require.NoError(t, won, "the send wins")
	require.Error(t, lost)
	assert.True(t, definedLoss(lost), "the conclusion gets a defined error, got %v", lost)
	s := f.state(t)
	assert.Equal(t, raceState{found: true, status: carecontract.StatusSent, versions: 1, documents: 1, mailings: 1}, s,
		"sent and not concluded")
}
