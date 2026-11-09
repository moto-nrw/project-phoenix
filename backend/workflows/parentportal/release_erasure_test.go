//go:build testhooks

package parentportal_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// pausedReleaseRecipients holds the real candidate read after it collected
// account IDs, before the fan-out opens its enqueue transaction.
type pausedReleaseRecipients struct {
	parentportal.ReleaseRecipientsPort
	collected chan struct{}
	resume    chan struct{}
}

func (p pausedReleaseRecipients) GuardianAccounts(ctx context.Context, school int64, group *int64) ([]int64, error) {
	accounts, err := p.ReleaseRecipientsPort.GuardianAccounts(ctx, school, group)
	close(p.collected)
	select {
	case <-p.resume:
		return accounts, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// pausedReleaseFanOut holds the real fan-out after its inserts, while the
// workflow still owns the enqueue transaction.
type pausedReleaseFanOut struct {
	parentportal.ReleaseFanOutPort
	inserted chan struct{}
	resume   chan struct{}
}

func (p pausedReleaseFanOut) EnqueueReleaseAccounts(ctx context.Context, organization, release int64, accounts []int64) error {
	if err := p.ReleaseFanOutPort.EnqueueReleaseAccounts(ctx, organization, release, accounts); err != nil {
		return err
	}
	close(p.inserted)
	select {
	case <-p.resume:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type releaseErasureFixture struct {
	db                    *bun.DB
	ctx                   context.Context
	chain                 testpkg.ParentChain
	organization, release int64
	deps                  parentportalcompose.Dependencies
	ports                 parentportal.DocumentPushPorts
}

func newReleaseErasureFixture(t *testing.T) releaseErasureFixture {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.EnsureTestTenant(t, db, testpkg.OwnTenant(t))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.ContextWithTenantRuntime(t.Context(), testpkg.TenantRuntime(t, db))
	organization := organizationOf(t, db, chain.TenantID)
	own := sharedDocumentTraeger{t: t, db: db, organization: organization}
	release := own.release(own.document("Hausordnung"), "parents", nil, nil)
	deps := documentInboxDependencies(t, db, &owners.CareContractTestMails{})
	people := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	schools, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	outbox, err := owners.NewEnrollmentFlowDelivery(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	pushes := &recordedPushes{}
	deps.DocumentNotifier = pushes
	deps.StudentRepo = owners.NewParentStudentReads(people.Student)
	return releaseErasureFixture{db: db, ctx: ctx, chain: chain, organization: organization, release: release, deps: deps,
		ports: parentportal.DocumentPushPorts{
			ReleaseNotices: owners.NewParentReleaseNotices(), ReleaseSchools: owners.NewParentReleaseSchools(schools),
			ReleaseRecipients: owners.NewParentReleaseRecipients(owners.NewParentGroupAudience(testpkg.NewDatabase(t, db))),
			ReleaseFanOut:     owners.ParentReleaseFanOut{Delivery: outbox}, TraegerDocumentNotifier: pushes,
			LockReleaseAccounts: owners.NewParentReleaseAccountLocks(testpkg.NewDatabase(t, db)),
		}}
}

func (f releaseErasureFixture) erase(t *testing.T) int {
	t.Helper()
	_, err := f.db.ExecContext(f.ctx, `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, f.chain.TenantID)
	require.NoError(t, err)
	_, err = f.db.ExecContext(f.ctx, `INSERT INTO platform.demo_school_states
		(name, tenant_id, seed_state, status, visitor_parent_account_id)
		VALUES (?, ?, '{}'::jsonb, 'ready', ?) ON CONFLICT (name) DO NOTHING`,
		fmt.Sprintf("release-erasure-%d", f.chain.TenantID), f.chain.TenantID, f.chain.AccountID)
	require.NoError(t, err)
	var erased int
	require.NoError(t, f.db.NewRaw(`SELECT platform.erase_closed_demo_visitors()`).Scan(f.ctx, &erased))
	return erased
}

func (f releaseErasureFixture) accountEntries(t *testing.T) int {
	t.Helper()
	var count int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM platform.push_outbox
		WHERE kind = 'traeger_document_release_account' AND related_entity_id = ? AND payload->>'account_id' = ?`,
		f.release, fmt.Sprint(f.chain.AccountID)).Scan(f.ctx, &count))
	return count
}

func (f releaseErasureFixture) accountErased(t *testing.T) bool {
	t.Helper()
	var erased bool
	require.NoError(t, f.db.NewRaw(`SELECT personal_data_erased_at IS NOT NULL FROM auth.accounts WHERE id = ?`,
		f.chain.AccountID).Scan(f.ctx, &erased))
	return erased
}

func waitForRelease(t *testing.T, ready <-chan struct{}) {
	t.Helper()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("release fan-out did not reach the pause")
	}
}

func startRelease(t *testing.T, f releaseErasureFixture) <-chan error {
	t.Helper()
	notices := parentportalcompose.NewDocumentNotices(f.deps, f.ports)
	done := make(chan error, 1)
	go func() { done <- notices.NotifyTraegerDocumentRelease(f.ctx, f.organization, f.release) }()
	return done
}

func finishRelease(t *testing.T, done *<-chan error) {
	t.Helper()
	select {
	case err := <-*done:
		*done = nil
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("release fan-out did not finish")
	}
}

func TestReleaseFanOutDropsCandidatesErasedAfterTheirRead(t *testing.T) {
	t.Parallel()
	f := newReleaseErasureFixture(t)
	pause := pausedReleaseRecipients{ReleaseRecipientsPort: f.ports.ReleaseRecipients, collected: make(chan struct{}), resume: make(chan struct{})}
	f.ports.ReleaseRecipients = pause
	done := startRelease(t, f)
	defer func() {
		if pause.resume != nil {
			close(pause.resume)
		}
		if done != nil {
			finishRelease(t, &done)
		}
	}()
	waitForRelease(t, pause.collected)
	require.Equal(t, 1, f.erase(t))
	require.True(t, f.accountErased(t), "the candidate's account has been anonymized")
	close(pause.resume)
	pause.resume = nil
	finishRelease(t, &done)
	assert.Zero(t, f.accountEntries(t), "a stale candidate must not recreate the erased account's entry")
}

func TestDemoErasureDefersUntilTheReleaseFanOutCommits(t *testing.T) {
	t.Parallel()
	f := newReleaseErasureFixture(t)
	pause := pausedReleaseFanOut{ReleaseFanOutPort: f.ports.ReleaseFanOut, inserted: make(chan struct{}), resume: make(chan struct{})}
	f.ports.ReleaseFanOut = pause
	done := startRelease(t, f)
	defer func() {
		if pause.resume != nil {
			close(pause.resume)
		}
		if done != nil {
			finishRelease(t, &done)
		}
	}()
	waitForRelease(t, pause.inserted)
	require.Zero(t, f.erase(t), "the in-flight fan-out keeps the whole order pending")
	require.False(t, f.accountErased(t), "the deferred erasure leaves the account unchanged")
	close(pause.resume)
	pause.resume = nil
	finishRelease(t, &done)
	require.Equal(t, 1, f.accountEntries(t))
	require.Equal(t, 1, f.erase(t))
	assert.Zero(t, f.accountEntries(t), "the next erasure sees and removes the committed entry")
}

// schoolRelease turns the fixture's release into a release for the demo
// school alone and keeps its candidate in a second, open school of the
// Träger, so the erasure keeps the account (#6342).
func (f *releaseErasureFixture) schoolRelease(t *testing.T) {
	t.Helper()
	own := sharedDocumentTraeger{t: t, db: f.db, organization: f.organization}
	f.release = own.release(own.document("Elternbrief"), "parents", f.chain.TenantID, nil)
	second := testpkg.CreateTestSchoolInOrganization(t, f.db, f.organization, "Zweite Schule")
	testpkg.EnsureAccountTenant(t, f.db, f.chain.AccountID, second)
}

// A release for the demo school read its candidates before the school was
// retired and its visitor erased. The account stays for its other school,
// yet the fan-out adds no entry for the retired school's release afterwards:
// no later erasure would find it.
func TestReleaseFanOutSkipsASchoolRetiredAfterItsRead(t *testing.T) {
	t.Parallel()
	f := newReleaseErasureFixture(t)
	f.schoolRelease(t)
	pause := pausedReleaseRecipients{ReleaseRecipientsPort: f.ports.ReleaseRecipients, collected: make(chan struct{}), resume: make(chan struct{})}
	f.ports.ReleaseRecipients = pause
	done := startRelease(t, f)
	defer func() {
		if pause.resume != nil {
			close(pause.resume)
		}
		if done != nil {
			finishRelease(t, &done)
		}
	}()
	waitForRelease(t, pause.collected)
	require.Equal(t, 1, f.erase(t))
	require.False(t, f.accountErased(t), "the account stays for its other school")
	close(pause.resume)
	pause.resume = nil
	finishRelease(t, &done)
	assert.Zero(t, f.accountEntries(t), "the retired school's release adds no entry after the erasure")
}

// While a fan-out of a release for the demo school is enqueueing, retiring
// the school waits for it; a retirement that cannot wait (lock timeout)
// leaves the school open, and the erasure after the commit finds every entry.
func TestRetiringTheSchoolWaitsForItsReleaseFanOut(t *testing.T) {
	t.Parallel()
	f := newReleaseErasureFixture(t)
	f.schoolRelease(t)
	pause := pausedReleaseFanOut{ReleaseFanOutPort: f.ports.ReleaseFanOut, inserted: make(chan struct{}), resume: make(chan struct{})}
	f.ports.ReleaseFanOut = pause
	done := startRelease(t, f)
	defer func() {
		if pause.resume != nil {
			close(pause.resume)
		}
		if done != nil {
			finishRelease(t, &done)
		}
	}()
	waitForRelease(t, pause.inserted)
	err := f.db.RunInTx(f.ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '100ms'`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, f.chain.TenantID)
		return err
	})
	require.ErrorContains(t, err, "lock timeout", "retiring the school waits for the fan-out of its release")
	close(pause.resume)
	pause.resume = nil
	finishRelease(t, &done)
	require.Equal(t, 1, f.accountEntries(t))
	require.Equal(t, 1, f.erase(t))
	assert.Zero(t, f.accountEntries(t), "the erasure removes the committed entry with the school's release")
}
