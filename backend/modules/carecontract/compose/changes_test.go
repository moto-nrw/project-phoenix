package compose_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// changeSchool is the school of a contract with a child that has an open
// change; it records what the office decided.
type changeSchool struct {
	request   int64
	readChild int64
	impact    *carecontract.ChangeImpact
	schools   []carecontract.School
	decisions []carecontract.ChangeDecision
	accesses  []carecontract.SchoolAccess
}

func (s *changeSchool) TraegerSchools(context.Context, int64) ([]carecontract.School, error) {
	return s.schools, nil
}

func (s *changeSchool) PendingChanges(_ context.Context, _ int64, childID int64, _ *calendar.Date) ([]carecontract.ChangeRequest, *carecontract.ChangeOutcome, bool, error) {
	s.readChild = childID
	return []carecontract.ChangeRequest{{ID: s.request, EffectiveFrom: calendar.NewDate(2100, 3, 1)}}, nil, false, nil
}

func (s *changeSchool) DecideChange(_ context.Context, decision carecontract.ChangeDecision) (*carecontract.ChangeImpact, error) {
	s.decisions = append(s.decisions, decision)
	return s.impact, nil
}

func (s *changeSchool) RecordAccess(_ context.Context, access carecontract.SchoolAccess) error {
	s.accesses = append(s.accesses, access)
	return nil
}

// bypassRuntime runs the Träger transaction as the test database's owner,
// whom no row-level policy applies to: only the queries' own filters keep
// the Träger apart (rule 25 of the brief, #5952).
type bypassRuntime struct{ uow tenant.UnitOfWork }

func newBypassRuntime(db *bun.DB) bypassRuntime {
	return bypassRuntime{uow: testpkg.PassthroughTenantRuntime().WithTraegerTransaction(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error { return fn(ctx, tx) })
		})}
}

func (r bypassRuntime) WithinSchool(ctx context.Context, _ int64, fn func(context.Context) error) error {
	return fn(ctx)
}

func (r bypassRuntime) WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	return tenant.WithinTraeger(tenant.WithUnitOfWork(ctx, r.uow), organizationID, fn)
}

type changeFixture struct {
	db           *bun.DB
	organization int64
	foreign      int64
	school       carecontract.School
	student      int64
	child        int64
	contract     int64
	request      int64
}

func newChangeFixture(t *testing.T, status string) changeFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) int64 {
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
			Name: name, Slug: fmt.Sprintf("traeger-wunsch-%d", testpkg.UniqueTestTenantID(t)),
		}).ID
		testpkg.OwnTraegerCareContracts(t, db, organization)
		return organization
	}
	f := changeFixture{db: db, organization: newTraeger("Träger Änderungswunsch"), foreign: newTraeger("Träger Änderungswunsch Fremd"),
		school: carecontract.School{ID: testpkg.UniqueTestTenantID(t), Name: "OGS Am Lindenhof"}, student: testpkg.UniqueTestTenantID(t), request: testpkg.UniqueTestTenantID(t), child: testpkg.UniqueTestTenantID(t)}
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id, student_id,
			status, sent_at, concluded_at, conclusion_method) VALUES (?, ?, ?, ?, ?, NOW(), NOW(), 'mit_anmeldung') RETURNING id`,
		f.organization, f.school.ID, f.child, f.student, status).Scan(ctx, &f.contract))
	return f
}

func (f changeFixture) changes(school *changeSchool) carecontract.ContractChangeDecisions {
	return compose.NewContractChanges(carecontract.ChangePorts{Schools: school, Requests: school, Access: school, Runtime: newBypassRuntime(f.db)})
}

func approval() carecontract.ChangeDecisionInput {
	day := calendar.NewDate(2100, 3, 1)
	return carecontract.ChangeDecisionInput{Approve: true, EffectiveFrom: &day, ExpectedVersion: "v1"}
}

// The office decides only for an active contract of its own Träger whose
// school still belongs to the Träger. Every check holds without the Träger
// policy: the Träger transaction runs as the database owner here.
func TestTheOfficeDecidesOnlyForAnActiveContractOfItsOwnTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	f := newChangeFixture(t, carecontract.StatusActive)
	ctx := context.Background()
	account := testpkg.CreateTestAccountWithPassword(t, f.db, fmt.Sprintf("wunsch-%d@test.local", testpkg.UniqueTestTenantID(t)), testpkg.NewTestPassword())
	own := carecontract.Reviewer{OrganizationID: f.organization, AccountID: account.ID}

	school := &changeSchool{request: f.request, schools: []carecontract.School{f.school}, impact: &carecontract.ChangeImpact{EffectiveFrom: *approval().EffectiveFrom}}
	result, err := f.changes(school).DecideContractChange(ctx, own, f.contract, f.request, approval())
	require.NoError(t, err)
	require.Len(t, school.decisions, 1)
	assert.Equal(t, f.student, school.decisions[0].StudentID, "the decision names the contract's child")
	assert.Equal(t, f.child, school.decisions[0].RequestChildID)
	assert.Equal(t, f.child, school.readChild)
	assert.Equal(t, school.impact, result.RosterImpact)
	assert.Equal(t, f.request, school.decisions[0].RequestID)
	assert.Equal(t, account.ID, school.decisions[0].AccountID)
	assert.Equal(t, "v1", school.decisions[0].ExpectedVersion)
	assert.Len(t, result.Pending, 1)
	require.Len(t, school.accesses, 1, "the decision is in the school's access log")
	assert.Equal(t, carecontract.SchoolAccessChanges, school.accesses[0].Data)
	assert.Equal(t, []int64{f.contract}, school.accesses[0].ContractIDs)
	assert.Equal(t, []int64{f.student}, school.accesses[0].StudentIDs)

	foreign := carecontract.Reviewer{OrganizationID: f.foreign, AccountID: account.ID}
	school = &changeSchool{request: f.request, schools: []carecontract.School{f.school}}
	_, err = f.changes(school).DecideContractChange(ctx, foreign, f.contract, f.request, approval())
	require.ErrorIs(t, err, carecontract.ErrContractNotFound, "another Träger finds the contract without RLS neither")
	_, err = f.changes(school).ContractChanges(ctx, foreign, f.contract, nil)
	require.ErrorIs(t, err, carecontract.ErrContractNotFound)
	assert.Empty(t, school.decisions)
	assert.Empty(t, school.accesses)

	moved := &changeSchool{request: f.request, schools: []carecontract.School{{ID: testpkg.UniqueTestTenantID(t), Name: "Andere OGS"}}}
	_, err = f.changes(moved).DecideContractChange(ctx, own, f.contract, f.request, approval())
	require.ErrorIs(t, err, carecontract.ErrSchoolOutsideTraeger, "a school of another Träger is not the office's to decide for")
	_, err = f.changes(moved).ContractChanges(ctx, own, f.contract, nil)
	require.ErrorIs(t, err, carecontract.ErrSchoolOutsideTraeger)
	assert.Empty(t, moved.decisions)
	assert.Empty(t, moved.accesses)
}

// Only an active contract follows a change with a version; on any other
// the office reads but does not decide.
func TestTheOfficeDoesNotDecideForAContractThatIsNotActive(t *testing.T) {
	t.Parallel()
	f := newChangeFixture(t, carecontract.StatusTerminated)
	account := testpkg.CreateTestAccountWithPassword(t, f.db, fmt.Sprintf("wunsch-ende-%d@test.local", testpkg.UniqueTestTenantID(t)), testpkg.NewTestPassword())
	own := carecontract.Reviewer{OrganizationID: f.organization, AccountID: account.ID}
	school := &changeSchool{request: f.request, schools: []carecontract.School{f.school}}

	_, err := f.changes(school).DecideContractChange(context.Background(), own, f.contract, f.request, approval())
	require.ErrorIs(t, err, carecontract.ErrContractNotActive)
	assert.Empty(t, school.decisions)
	read, err := f.changes(school).ContractChanges(context.Background(), own, f.contract, nil)
	require.NoError(t, err)
	assert.False(t, read.Decidable)
}

// A rejection needs a reason and no day; an approval needs its day.
func TestTheOfficeDecisionNeedsItsDayOrItsReason(t *testing.T) {
	t.Parallel()
	f := newChangeFixture(t, carecontract.StatusActive)
	account := testpkg.CreateTestAccountWithPassword(t, f.db, fmt.Sprintf("wunsch-grund-%d@test.local", testpkg.UniqueTestTenantID(t)), testpkg.NewTestPassword())
	own := carecontract.Reviewer{OrganizationID: f.organization, AccountID: account.ID}
	school := &changeSchool{request: f.request, schools: []carecontract.School{f.school}}
	changes := f.changes(school)

	_, err := changes.DecideContractChange(context.Background(), own, f.contract, f.request, carecontract.ChangeDecisionInput{Approve: true})
	require.ErrorIs(t, err, carecontract.ErrChangeInvalid)
	_, err = changes.DecideContractChange(context.Background(), own, f.contract, f.request, carecontract.ChangeDecisionInput{Reason: "  "})
	require.ErrorIs(t, err, carecontract.ErrChangeReasonRequired)
	assert.Empty(t, school.decisions)
}
