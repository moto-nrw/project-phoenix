package contracttest_test

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/parentrequests"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The Träger-Büro decides the parents' change through the same review as
// the school (#5979, ADR 0163). These tests run the real review over the
// suite's database: once as the school's staff, once through the
// care-contract binding of the Träger-Büro, whose context carries no school
// permission at all.

// traegerDecisionReasonPolicy is the school's rule for reasons on parent
// requests, which the office's decision reads like the school's route.
const traegerDecisionReasonPolicy = "operations.parent_request_reason_policy"

type traegerDecisionFixture struct {
	env     *decisionTestEnv
	svc     careplan.OfferingChangeCapability
	office  owners.CareContractChangeRequests
	request int64
	fx      *offeringChangeFixture
}

func newTraegerDecisionFixture(t *testing.T, env *decisionTestEnv, svc careplan.OfferingChangeCapability, label string) traegerDecisionFixture {
	t.Helper()
	fx := setupOfferingChangeFixture(t, env, label)
	// The school asks staff for a reason on a rejection only.
	env.settings.mu.Lock()
	env.settings.stringValues[traegerDecisionReasonPolicy] = parentrequests.ReasonPolicyGuardians
	env.settings.mu.Unlock()
	row, err := testpkg.TenantQuery1(t, offeringChangeAdminContext(t), env.db, svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}}},
	})
	require.NoError(t, err)
	return traegerDecisionFixture{env: env, svc: svc, office: owners.CareContractChangeRequests{Changes: svc, Settings: env.settings},
		request: row.ID, fx: fx}
}

// school decides as the school's review route does.
func (f traegerDecisionFixture) school(ctx context.Context) error {
	day := f.fx.switchDate
	_, err := f.svc.Decide(ctx, careplan.OfferingChangeDecisionInput{RequestID: f.request, Approve: true, ReviewedBy: f.env.creatorID, EffectiveFrom: &day})
	return err
}

// traeger decides as the Träger-Büro's route does, for the given child.
func (f traegerDecisionFixture) traeger(ctx context.Context, studentID int64) error {
	day := f.fx.switchDate
	_, err := f.office.DecideChange(ctx, api.ContractChangeDecisionForTest{
		ChangeDecisionInput: api.ContractChangeDecisionInputForTest{Approve: true, EffectiveFrom: &day},
		StudentID:           studentID, RequestChildID: f.fx.childID, RequestID: f.request, AccountID: f.env.creatorID,
	})
	return err
}

// bookings answers the child's bookings with the offerings named by their
// role in the switch, so two children's switches compare.
func (f traegerDecisionFixture) bookings(t *testing.T) []string {
	t.Helper()
	var rows []struct {
		CareOfferingID int64          `bun:"care_offering_id"`
		Days           string         `bun:"days"`
		ValidFrom      *calendar.Date `bun:"valid_from"`
		ValidUntil     *calendar.Date `bun:"valid_until"`
	}
	require.NoError(t, f.env.db.NewRaw(`SELECT care_offering_id, COALESCE(manual_selected_days::text, '') AS days, valid_from, valid_until
		FROM enrollment.care_offering_bookings WHERE tenant_id = ? AND request_child_id = ?`, testpkg.Tenant(t), f.fx.childID).Scan(context.Background(), &rows))
	role := map[int64]string{f.fx.oldOffering.ID: "bisher", f.fx.newOffering.ID: "neu"}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		from, until := "", ""
		if row.ValidFrom != nil {
			from = row.ValidFrom.String()
		}
		if row.ValidUntil != nil {
			until = row.ValidUntil.String()
		}
		result = append(result, fmt.Sprintf("%s %s %s..%s", role[row.CareOfferingID], strings.ReplaceAll(row.Days, " ", ""), from, until))
	}
	sort.Strings(result)
	return result
}

// inTraegerDecisionTx runs fn in the test tenant's transaction.
func inTraegerDecisionTx(t *testing.T, ctx context.Context, db *bun.DB, fn func(context.Context) error) error {
	t.Helper()
	return testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error { return fn(txCtx) })
}

func (f traegerDecisionFixture) status(t *testing.T) string {
	t.Helper()
	var status string
	require.NoError(t, f.env.db.NewRaw(`SELECT status FROM enrollment.offering_change_requests WHERE tenant_id = ? AND id = ?`,
		testpkg.Tenant(t), f.request).Scan(context.Background(), &status))
	return status
}

// The Träger-Büro's approval books exactly what the school's approval books
// for the same request: the contract's version follows these bookings
// (AmendFromBookings reads nothing else), so both lead to the same version.
// The office's context carries no school permission; only the binding gives
// it the school's reach.
func TestTraegerApprovalBooksTheSameSwitchAsTheSchoolsApproval(t *testing.T) {
	t.Parallel()
	env := setupDecisionTest(t)
	svc := newOfferingChangeServiceForTest(t, env)
	bySchool := newTraegerDecisionFixture(t, env, svc, "Schule")
	byOffice := newTraegerDecisionFixture(t, env, svc, "Buero")
	before := bySchool.bookings(t)
	require.Equal(t, before, byOffice.bookings(t), "both children start alike")

	require.NoError(t, inTraegerDecisionTx(t, offeringChangeAdminContext(t), env.db, bySchool.school))
	require.NoError(t, inTraegerDecisionTx(t, testpkg.Ctx(t), env.db, func(ctx context.Context) error {
		return byOffice.traeger(ctx, byOffice.fx.studentID)
	}))

	assert.Equal(t, careplan.OfferingChangeApproved, bySchool.status(t))
	assert.Equal(t, careplan.OfferingChangeApproved, byOffice.status(t))
	after := bySchool.bookings(t)
	assert.NotEqual(t, before, after, "the approval switched the booking")
	assert.Equal(t, after, byOffice.bookings(t), "the office books the same switch from the same day")

	// Without the binding the same context reaches no child of the school.
	plain := newTraegerDecisionFixture(t, env, svc, "Ohne")
	err := inTraegerDecisionTx(t, testpkg.Ctx(t), env.db, plain.school)
	require.ErrorIs(t, err, careplan.ErrOfferingChangeForbidden)
	assert.Equal(t, careplan.OfferingChangePending, plain.status(t))
}

// A request of another child is not the contract's: Care Plan reports it
// missing under the request lock, and nothing is booked or decided.
func TestTraegerDecisionOfAnotherChildsRequestChangesNothing(t *testing.T) {
	t.Parallel()
	env := setupDecisionTest(t)
	svc := newOfferingChangeServiceForTest(t, env)
	f := newTraegerDecisionFixture(t, env, svc, "FremdesKind")
	other := setupOfferingChangeFixture(t, env, "Vertragskind")
	before := f.bookings(t)

	err := inTraegerDecisionTx(t, testpkg.Ctx(t), env.db, func(ctx context.Context) error {
		return f.traeger(ctx, other.studentID)
	})
	require.ErrorIs(t, err, api.ContractChangeNotFoundForTest())
	assert.Equal(t, careplan.OfferingChangePending, f.status(t))
	assert.Equal(t, before, f.bookings(t))
}

// The school and the Träger-Büro approve the same request at the same time,
// on two connections: the second waits for the first one's request lock and
// then finds the request decided. Exactly one approval books the switch.
func TestSchoolAndTraegerApprovingAtOnceApplyTheChangeOnce(t *testing.T) {
	t.Parallel()
	env := setupDecisionTest(t)
	svc := newOfferingChangeServiceForTest(t, env)
	for _, first := range []string{"school", "traeger"} {
		t.Run(first+" first", func(t *testing.T) {
			f := newTraegerDecisionFixture(t, env, svc, "Gleichzeitig"+first)
			decide := map[string]func(context.Context) error{
				"school":  f.school,
				"traeger": func(ctx context.Context) error { return f.traeger(ctx, f.fx.studentID) },
			}
			contexts := map[string]context.Context{"school": offeringChangeAdminContext(t), "traeger": testpkg.Ctx(t)}
			second := "traeger"
			if first == "traeger" {
				second = "school"
			}
			ctx, cancel := context.WithTimeout(contexts[first], 20*time.Second)
			defer cancel()
			secondCtx, cancelSecond := context.WithTimeout(contexts[second], 20*time.Second)
			defer cancelSecond()
			done := make(chan error, 1)
			firstErr := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
				if err := decide[first](txCtx); err != nil {
					return err
				}
				var pid int
				require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &pid))
				go func() {
					done <- testpkg.WithTenantTx(t, secondCtx, env.db, testpkg.Tenant(t), func(writeCtx context.Context, _ bun.Tx) error {
						return decide[second](writeCtx)
					})
				}()
				waitForRequestWriter(t, txCtx, tx, pid)
				return nil
			})
			require.NoError(t, firstErr)
			secondErr := awaitRequestWriter(t, secondCtx, done)
			require.ErrorIs(t, secondErr, careplan.ErrOfferingChangeNotPending, "the second approval finds the request decided")
			assert.Equal(t, careplan.OfferingChangeApproved, f.status(t))
			news := 0
			for _, booking := range f.bookings(t) {
				if strings.HasPrefix(booking, "neu ") {
					news++
				}
			}
			assert.Equal(t, 1, news, "the switch is booked once")
		})
	}
}

// Another enrollment of the same child must neither appear nor be decided
// from this contract. The enrollment pin is independent of the student pin.
func TestTraegerChangeRequiresTheContractsEnrollment(t *testing.T) {
	t.Parallel()
	env := setupDecisionTest(t)
	svc := newOfferingChangeServiceForTest(t, env)
	f := newTraegerDecisionFixture(t, env, svc, "Anmeldung")
	other := setupOfferingChangeFixture(t, env, "AndereAnmeldung")
	_, err := env.db.NewRaw(`UPDATE enrollment.request_children SET created_student_id = ?
  WHERE tenant_id = ? AND id = ?`, f.fx.studentID, testpkg.Tenant(t), other.childID).Exec(context.Background())
	require.NoError(t, err)
	before := f.bookings(t)
	require.NoError(t, inTraegerDecisionTx(t, testpkg.Ctx(t), env.db, func(ctx context.Context) error {
		pending, _, _, err := f.office.PendingChanges(ctx, f.fx.studentID, other.childID, nil)
		require.NoError(t, err)
		assert.Empty(t, pending)
		pending, _, _, err = f.office.PendingChanges(ctx, f.fx.studentID, f.fx.childID, nil)
		require.NoError(t, err)
		require.Len(t, pending, 1)
		assert.Equal(t, f.request, pending[0].ID)
		return nil
	}))
	err = inTraegerDecisionTx(t, testpkg.Ctx(t), env.db, func(ctx context.Context) error {
		_, err := f.office.DecideChange(ctx, api.ContractChangeDecisionForTest{
			ChangeDecisionInput: api.ContractChangeDecisionInputForTest{Approve: true, EffectiveFrom: &f.fx.switchDate},
			StudentID:           f.fx.studentID, RequestChildID: other.childID, RequestID: f.request, AccountID: env.creatorID,
		})
		return err
	})
	require.ErrorIs(t, err, api.ContractChangeNotFoundForTest())
	assert.Equal(t, careplan.OfferingChangePending, f.status(t))
	assert.Equal(t, before, f.bookings(t))
}

func TestTraegerChangePreviewUsesTheConfirmedDate(t *testing.T) {
	t.Parallel()
	env := setupDecisionTest(t)
	svc := newOfferingChangeServiceForTest(t, env)
	f := newTraegerDecisionFixture(t, env, svc, "Vorschautag")
	later := f.fx.switchDate.AddDays(7)
	_, err := env.db.NewRaw(`UPDATE enrollment.care_offering_bookings SET valid_until = ?
  WHERE tenant_id = ? AND request_child_id = ? AND care_offering_id = ?`,
		later, testpkg.Tenant(t), f.fx.childID, f.fx.oldOffering.ID).Exec(context.Background())
	require.NoError(t, err)
	require.NoError(t, inTraegerDecisionTx(t, testpkg.Ctx(t), env.db, func(ctx context.Context) error {
		initial, _, _, err := f.office.PendingChanges(ctx, f.fx.studentID, f.fx.childID, &f.fx.switchDate)
		require.NoError(t, err)
		require.Len(t, initial, 1)
		assert.True(t, containsChangeOffering(initial[0].Lines, f.fx.oldOffering.ID), "the old booking still exists before its end")
		preview, _, _, err := f.office.PendingChanges(ctx, f.fx.studentID, f.fx.childID, &later)
		require.NoError(t, err)
		require.Len(t, preview, 1)
		assert.False(t, containsChangeOffering(preview[0].Lines, f.fx.oldOffering.ID), "the selected date's comparison must not show an expired booking")
		assert.True(t, containsChangeOffering(preview[0].Lines, f.fx.newOffering.ID))
		return nil
	}))
	assert.Equal(t, careplan.OfferingChangePending, f.status(t), "preview does not decide")
}

func containsChangeOffering(lines []api.ContractChangeLineForTest, offeringID int64) bool {
	for _, line := range lines {
		if line.CareOfferingID == offeringID {
			return true
		}
	}
	return false
}
