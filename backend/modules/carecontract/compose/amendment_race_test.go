package compose_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// raceFixture is a Träger with an active contract whose booked care changed,
// and the conclusion that amends it.
type raceFixture struct {
	db           *bun.DB
	ctx          context.Context
	organization int64
	contract     int64
	conclusion   carecontract.Conclusion
}

func newRaceFixture(t *testing.T) raceFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, db))
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Änderung Sperre", Slug: fmt.Sprintf("traeger-sperre-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerCareContracts(t, db, organization)
	school := changedSchool{school: carecontract.School{ID: testpkg.UniqueTestTenantID(t), Name: "OGS Am Lindenhof"},
		child: testpkg.UniqueTestTenantID(t), offering: testpkg.UniqueTestTenantID(t)}
	contract := activeContract(t, ctx, organization, school)
	conclusion := compose.NewConclusion(
		compose.ContractPorts{Letterheads: letterheadDouble{}, Post: &postDouble{}, Runtime: runtimeDouble{}, Access: school,
			Ledger: ledgerDouble{}, Links: testSigningLinks()},
		compose.IntakePorts{Schools: school, Applications: school, Bookings: school, Access: school, Runtime: runtimeDouble{}, Modes: modesDouble{}, Mail: noMailBlocks{}})
	return raceFixture{db: db, ctx: ctx, organization: organization, contract: contract, conclusion: conclusion}
}

func (f raceFixture) versions(t *testing.T) int {
	t.Helper()
	var count int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_versions WHERE contract_id = ?`, f.contract).Scan(f.ctx, &count))
	return count
}

// waitForLockWaiter waits until a statement of this Träger's amendment waits
// on a lock the other transaction holds: the row lock of the contract, or,
// without it, the key lock its version insert needs.
func (f raceFixture) waitForLockWaiter(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting int
		err := f.db.NewRaw(`SELECT COUNT(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'
				AND query LIKE '%care_contract.%' AND query LIKE ?`,
			fmt.Sprintf("%%%d%%", f.organization)).Scan(f.ctx, &waiting)
		return err == nil && waiting > 0
	}, 20*time.Second, 20*time.Millisecond, "the amendment never waited on the contract's row lock")
}

// Invariant (#4681): a new version follows only an active contract, under the
// same row lock of the contract that the conclusion takes (#4680). A second
// transaction ends the contract while the amendment run reads it: the
// amendment waits for the lock, sees the ended contract and adds no version.
// Without the lock it reads the contract as still active from before the
// commit and appends a version to an ended contract.
func TestAmendmentWaitsForTheContractLockAndLeavesAnEndedContractAlone(t *testing.T) {
	t.Parallel()
	f := newRaceFixture(t)
	locked, release, ended := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		ended <- f.db.RunInTx(f.ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_org_id', ?, true)`, fmt.Sprint(f.organization)); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT id FROM care_contract.contracts WHERE id = ? FOR UPDATE`, f.contract); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE care_contract.contracts SET status = 'beendet' WHERE id = ?`, f.contract); err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked

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
	close(release)
	require.NoError(t, <-ended)

	got := <-amended
	require.NoError(t, got.err)
	assert.Empty(t, got.run.Amended, "the ended contract gets no version")
	assert.Equal(t, 1, f.versions(t), "the contract keeps its only version")
}

// Invariant (#4681): two runs at the same time add the next version once.
// The second waits for the contract's lock and finds its predecessor already
// superseded.
func TestTwoParallelAmendmentRunsAddTheVersionOnce(t *testing.T) {
	t.Parallel()
	f := newRaceFixture(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	amended := make(chan int, 2)
	for range 2 {
		go func() {
			<-start
			run, err := f.conclusion.AmendFromBookings(f.ctx, f.organization)
			amended <- len(run.Amended)
			results <- err
		}()
	}
	close(start)
	total := 0
	for range 2 {
		require.NoError(t, <-results)
		total += <-amended
	}
	assert.Equal(t, 1, total, "exactly one run amends")
	assert.Equal(t, 2, f.versions(t), "one new version, no duplicate")
}
