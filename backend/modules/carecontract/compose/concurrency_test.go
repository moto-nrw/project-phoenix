package compose_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// holdingLetterhead holds the first caller inside the letterhead, while its
// transaction keeps whatever it locked, until the test releases it.
type holdingLetterhead struct {
	first   atomic.Bool
	entered chan struct{}
	release chan struct{}
}

func (h *holdingLetterhead) Letterhead(context.Context, carecontract.Reviewer) (carecontract.Letterhead, error) {
	if h.first.CompareAndSwap(false, true) {
		close(h.entered)
		<-h.release
	}
	return carecontract.Letterhead{SenderText: "Ganztag im Quartier e.V.\nAm Markt 4", ContactName: "Paula Kern"}, nil
}

// waitingPost answers every mail it enqueued as still waiting, safe for two
// transactions at once.
type waitingPost struct {
	mu   sync.Mutex
	next int64
	ids  []int64
}

func (p *waitingPost) EnqueueContractMail(context.Context, carecontract.ContractMail) (int64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next++
	p.ids = append(p.ids, p.next)
	return p.next, nil
}

// ShareMailBlockLimit shares nothing: no provider is asked.
func (p *waitingPost) ShareMailBlockLimit(ctx context.Context) context.Context { return ctx }

// ContractMailBlock holds no address back.
func (p *waitingPost) ContractMailBlock(context.Context, int64, string, string) (string, error) {
	return "", nil
}

func (p *waitingPost) ContractMailStates(context.Context, int64, int64) ([]carecontract.MailState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	states := make([]carecontract.MailState, len(p.ids))
	for i, id := range p.ids {
		states[i] = carecontract.MailState{OutboxID: id, State: carecontract.MailQueued}
	}
	return states, nil
}

type sendOutcome struct {
	result carecontract.SendResult
	err    error
}

// TestConcurrentSendsOfOneDraftLetExactlyOneTransitionWin pins the row lock
// of a contract (Contract with forUpdate): two Träger transactions send the
// same draft at the same time. The first holds the contract while it renders
// the PDF; the second must wait for it and then find the contract sent with
// its mail still waiting. Exactly one transition entwurf → versendet wins,
// one PDF is frozen, one mail goes out; the other send ends with the defined
// reason mail_pending, never with a database error. Without the lock the
// second reads the stale draft, renders again and fails on the frozen PDF.
func TestConcurrentSendsOfOneDraftLetExactlyOneTransitionWin(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, db))
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Vertrag Rennen", Slug: fmt.Sprintf("traeger-vertrag-rennen-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerCareContracts(t, db, organization)
	account := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("rennen-%d@test.local", testpkg.UniqueTestTenantID(t)), testpkg.NewTestPassword())
	reviewer := carecontract.Reviewer{OrganizationID: organization, AccountID: account.ID}
	accesses := 0
	school := schoolDouble{school: carecontract.School{ID: testpkg.UniqueTestTenantID(t), Name: "OGS Am Lindenhof"},
		child: testpkg.UniqueTestTenantID(t), offering: testpkg.UniqueTestTenantID(t), accesses: &accesses}
	intake := compose.NewIntake(compose.IntakePorts{Schools: school, Applications: school, Bookings: school, Access: school,
		Runtime: runtimeDouble{}, Modes: school, Mail: noMailBlocks{}, Letterheads: letterheadDouble{}})
	terms := carecontract.TemplateInput{TermKind: carecontract.TermOpenEnded, NoticeMonths: 1, NoticeTo: carecontract.NoticeMonthEnd}
	for _, offering := range []int64{school.offering, school.offering + 1, school.offering + 2} {
		_, err := intake.SaveTemplate(ctx, reviewer, school.school.ID, offering, terms)
		require.NoError(t, err)
	}
	created, err := intake.CreateContracts(ctx, reviewer, []carecontract.CandidateKey{{SchoolID: school.school.ID, RequestChildID: school.child}})
	require.NoError(t, err)
	require.Len(t, created.Created, 1)
	id := created.Created[0].ID

	letterhead := &holdingLetterhead{entered: make(chan struct{}), release: make(chan struct{})}
	post := &waitingPost{next: testpkg.UniqueTestTenantID(t)}
	contracts := compose.NewContracts(compose.ContractPorts{Letterheads: letterhead, Post: post, Runtime: runtimeDouble{}, Access: school,
		Ledger: ledgerDouble{}, Links: carecontract.SigningLinks{Tokens: &tokenDouble{}, ParentsURL: "http://parents.localhost", Fingerprint: sha256Hex}})
	send := func(out chan<- sendOutcome) {
		var outcome sendOutcome
		outcome.err = tenant.WithinTraeger(ctx, organization, func(txCtx context.Context) error {
			var err error
			outcome.result, err = contracts.SendContracts(txCtx, reviewer, []int64{id})
			return err
		})
		out <- outcome
	}
	first, second := make(chan sendOutcome, 1), make(chan sendOutcome, 1)
	go send(first)
	select {
	case <-letterhead.entered:
	case <-time.After(30 * time.Second):
		t.Fatal("the first send never reached the letterhead")
	}
	go send(second)
	// Release the first once the second waits on a lock of the contract,
	// or once it finished: with the row lock it waits, without it it runs
	// through on the stale draft.
	var secondDone *sendOutcome
	require.Eventually(t, func() bool {
		select {
		case outcome := <-second:
			secondDone = &outcome
			return true
		default:
		}
		var waiting int
		err := db.NewRaw(`SELECT COUNT(*) FROM pg_stat_activity WHERE datname = current_database()
			AND wait_event_type = 'Lock' AND query LIKE '%care_contract.contract%'`).Scan(context.Background(), &waiting)
		return err == nil && waiting > 0
	}, 30*time.Second, 20*time.Millisecond, "the second send either waits or finishes")
	close(letterhead.release)
	outcomes := []sendOutcome{<-first}
	if secondDone != nil {
		outcomes = append(outcomes, *secondDone)
	} else {
		outcomes = append(outcomes, <-second)
	}

	// Exactly one transition wins; the other send ends with the defined
	// reason, never with a database error.
	sent, pending := 0, 0
	for _, outcome := range outcomes {
		require.NoError(t, outcome.err, "a concurrent send gets a defined reason, not a database error")
		sent += len(outcome.result.Sent)
		for _, skip := range outcome.result.Skipped {
			assert.Equal(t, carecontract.SendSkip{ID: id, Reason: carecontract.SkipMailPending}, skip)
			pending++
		}
	}
	assert.Equal(t, 1, sent, "exactly one send wins the transition entwurf → versendet")
	assert.Equal(t, 1, pending, "the other send finds the mail still waiting")
	assert.Equal(t, []int64{id}, outcomes[0].result.Sent, "the send that held the lock first wins")

	var documents, mailings int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_documents WHERE contract_id = ?`, id).
		Scan(context.Background(), &documents))
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_mailings WHERE contract_id = ?`, id).
		Scan(context.Background(), &mailings))
	assert.Equal(t, 1, documents, "one frozen PDF")
	assert.Equal(t, 1, mailings, "one mail")
}
