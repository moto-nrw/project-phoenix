package compose_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// holdingRuntime keeps the Träger transaction of the termination open
// after its work, with the contract's row lock, until release closes.
type holdingRuntime struct {
	runtimeDouble
	held, release chan struct{}
}

func (r holdingRuntime) WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	return tenant.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		err := fn(txCtx)
		close(r.held)
		<-r.release
		return err
	})
}

type noticeTemplatesDouble struct{}

func (noticeTemplatesDouble) NoticeTemplate(context.Context, int64, string) (string, error) {
	return "Kündigung für {Kind} zum {Vertragsende}. {Träger}", nil
}

type noticePostDouble struct{ mails int }

func (p *noticePostDouble) EnqueueNoticeMail(context.Context, carecontract.NoticeMail) (int64, error) {
	p.mails++
	return int64(p.mails), nil
}

// Invariant (#4682): a termination and an amendment of the same contract
// at once change it once. The real termination holds the contract's row
// lock; the amendment run waits for it, sees gekuendigt and adds no
// version. Without the lock in the amendment (lockedTimeline) the run reads
// the contract as still aktiv and appends a version to a terminated
// contract, and this test goes red.
func TestTerminationHoldsTheLockAndTheAmendmentAddsNoVersion(t *testing.T) {
	t.Parallel()
	f := newRaceFixture(t)
	_, err := f.db.ExecContext(f.ctx, `INSERT INTO care_contract.contract_parties (organization_id, contract_id, first_name, last_name, email)
		VALUES (?, ?, 'Erika', 'Muster', 'erika@example.org')`, f.organization, f.contract)
	require.NoError(t, err)
	var received calendar.Date
	require.NoError(t, f.db.NewRaw(`SELECT (concluded_at AT TIME ZONE 'Europe/Berlin')::date FROM care_contract.contracts WHERE id = ?`,
		f.contract).Scan(f.ctx, &received))
	account := testpkg.CreateTestAccountWithPassword(t, f.db, fmt.Sprintf("kuendigung-%d@test.local", testpkg.UniqueTestTenantID(t)),
		testpkg.NewTestPassword()).ID

	runtime := holdingRuntime{held: make(chan struct{}), release: make(chan struct{})}
	school := changedSchool{school: carecontract.School{ID: 0, Name: "OGS Am Lindenhof"}}
	notices := compose.NewNotices(compose.ContractPorts{Letterheads: letterheadDouble{}, Post: &postDouble{}, Runtime: runtime, Access: school},
		compose.NoticePorts{Templates: noticeTemplatesDouble{}, Post: &noticePostDouble{}})
	terminated := make(chan error, 1)
	go func() {
		_, err := notices.DeclareNotice(f.ctx, carecontract.Reviewer{OrganizationID: f.organization, AccountID: account}, f.contract,
			carecontract.NoticeInput{Kind: carecontract.NoticeTermination, TerminationKind: carecontract.TerminationOrdinary, ReceivedOn: received})
		terminated <- err
	}()
	select {
	case <-runtime.held:
	case <-time.After(30 * time.Second):
		t.Fatal("the termination never reached its commit")
	}

	type result struct {
		run carecontract.AmendmentRun
		err error
	}
	amended := make(chan result, 1)
	go func() {
		run, err := f.conclusion.AmendFromBookings(f.ctx, f.organization)
		amended <- result{run: run, err: err}
	}()
	f.waitForLockWaiter(t)
	close(runtime.release)
	require.NoError(t, <-terminated)

	got := <-amended
	require.NoError(t, got.err)
	assert.Empty(t, got.run.Amended, "the terminated contract gets no version")
	assert.Equal(t, 1, f.versions(t), "the contract keeps its only version")
	var status string
	require.NoError(t, f.db.NewRaw(`SELECT status FROM care_contract.contracts WHERE id = ?`, f.contract).Scan(f.ctx, &status))
	assert.Equal(t, carecontract.StatusTerminated, status)
}
