package contracttest_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The Einladen dialog asks before an invitation goes out whether the address
// belongs to a former employment at this school (ADR 0021, #3402). The answer
// must match what accepting the invitation does: the latest retired
// membership of the account's person at this school, unless the automatic
// deletion of staff erased it.

func TestFormerEmployment_ReportsRetiredEmploymentAtThisSchool(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t, testpkg.SetupTestDB(t))
	emailAddr, _, staffID := formerEmploymentStaff(t, sc, "Frueher", "Hier")
	require.NoError(t, testpkg.TenantCommand3(t, sc.ctx, sc.db, sc.svc.OffboardStaff, staffID, staffID, "test-admin"))
	setMembershipPeriod(t, sc.db, staffID, "2024-08-01", "2026-07-31")

	former, found, err := formerEmploymentLookup(t, sc.db)(sc.ctx, " "+emailAddr+" ")
	require.NoError(t, err)
	require.True(t, found, "a retired membership at this school is a former employment")
	assert.Equal(t, calendar.NewDate(2024, time.August, 1), former.StartedOn)
	assert.Equal(t, calendar.NewDate(2026, time.July, 31), former.EndedOn)
	// The part keys are the wire contract the Einladen dialog names.
	assert.Equal(t, []string{"master_data", "qualifications", "work_time", "past_shifts_and_absences"},
		former.Resumes, "ADR 0021: what the continued employment brings back")
	assert.Equal(t, []string{"staff_documents", "future_shifts_and_absences", "groups_classes_substitutions"},
		former.NotReturning, "ADR 0021, decision 5: named, never offered")
}

func TestFormerEmployment_UnknownToOtherSchools(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t, testpkg.SetupTestDB(t))
	emailAddr, _, staffID := formerEmploymentStaff(t, sc, "Andere", "Schule")
	require.NoError(t, testpkg.TenantCommand3(t, sc.ctx, sc.db, sc.svc.OffboardStaff, staffID, staffID, "test-admin"))

	otherTenant, _ := testpkg.CreateTestTenant(t, sc.db)
	_, found, err := formerEmploymentLookup(t, sc.db)(tenant.WithTenantID(sc.ctx, otherTenant), emailAddr)
	require.NoError(t, err)
	assert.False(t, found, "a former employment at another school is unknown here")
}

func TestFormerEmployment_SilentWithoutFormerEmployment(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t, testpkg.SetupTestDB(t))
	lookup := formerEmploymentLookup(t, sc.db)

	unknown := fmt.Sprintf("former-unknown-%d@test.local", time.Now().UnixNano())
	_, found, err := lookup(sc.ctx, unknown)
	require.NoError(t, err)
	assert.False(t, found, "an address without an account")

	accountOnly := fmt.Sprintf("former-account-%d@test.local", time.Now().UnixNano())
	testpkg.CreateTestAccountWithPassword(t, sc.db, accountOnly, offboardingCredential("Account", "123"))
	_, found, err = lookup(sc.ctx, accountOnly)
	require.NoError(t, err)
	assert.False(t, found, "an account without a person here answers exactly like no account")

	live, _, _ := formerEmploymentStaff(t, sc, "Noch", "Da")
	_, found, err = lookup(sc.ctx, live)
	require.NoError(t, err)
	assert.False(t, found, "a live employment is not a former one")
}

func TestFormerEmployment_NamesTheLatestOfSeveral(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t, testpkg.SetupTestDB(t))
	emailAddr, personID, olderID := formerEmploymentStaff(t, sc, "Zwei", "Mal")
	retireMembership(t, sc.db, olderID, "2022-09-01", "2023-07-31")
	latestID := testpkg.CreateTestStaffForPerson(t, sc.db, personID).ID
	retireMembership(t, sc.db, latestID, "2024-08-01", "2025-07-31")

	former, found, err := formerEmploymentLookup(t, sc.db)(sc.ctx, emailAddr)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, calendar.NewDate(2024, time.August, 1), former.StartedOn)
	assert.Equal(t, calendar.NewDate(2025, time.July, 31), former.EndedOn)
}

// An erased latest membership is not resumed by the acceptance, which then
// starts a new employment; the preview must not promise otherwise.
func TestFormerEmployment_SilentWhenLatestWasErased(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t, testpkg.SetupTestDB(t))
	emailAddr, _, staffID := formerEmploymentStaff(t, sc, "Ge", "Loescht")
	retireMembership(t, sc.db, staffID, "2020-08-01", "2021-07-31")
	_, err := sc.db.ExecContext(context.Background(),
		`UPDATE users.staff_school_memberships SET personal_data_erased_at = '2024-08-01' WHERE id = ?`, staffID)
	require.NoError(t, err)

	_, found, err := formerEmploymentLookup(t, sc.db)(sc.ctx, emailAddr)
	require.NoError(t, err)
	assert.False(t, found)
}

// formerEmploymentLookup is the composed Identity & Access module's question
// of the Einladen dialog. This package may not name the module's contract,
// so the test reaches it through the root's test composition.
func formerEmploymentLookup(t *testing.T, db *bun.DB) func(context.Context, string) (formerEmploymentAnswer, bool, error) {
	t.Helper()
	module, err := api.NewTestIdentityAccess(testpkg.NewDatabase(t, db), slog.Default())
	require.NoError(t, err)
	// The question reads on the caller's school transaction, as the
	// request's tenant transaction middleware supplies it.
	return func(ctx context.Context, email string) (formerEmploymentAnswer, bool, error) {
		var answer formerEmploymentAnswer
		var found bool
		err := testpkg.WithinTenantContext(t, ctx, db, tenant.FromContext(ctx), func(txCtx context.Context) error {
			former, ok, findErr := module.FindFormerEmployment(txCtx, email)
			answer = formerEmploymentAnswer{StartedOn: former.StartedOn, EndedOn: former.EndedOn}
			for _, part := range former.Resumes {
				answer.Resumes = append(answer.Resumes, string(part))
			}
			for _, part := range former.NotReturning {
				answer.NotReturning = append(answer.NotReturning, string(part))
			}
			found = ok
			return findErr
		})
		return answer, found, err
	}
}

type formerEmploymentAnswer struct {
	StartedOn, EndedOn    calendar.Date
	Resumes, NotReturning []string
}

// formerEmploymentStaff creates a staff member who signs in with an account
// at this school and returns the address, the person and the membership.
func formerEmploymentStaff(t *testing.T, sc *offboardingScenario, firstName, lastName string) (string, int64, int64) {
	t.Helper()
	emailAddr := fmt.Sprintf("former-%s-%d@test.local", firstName, time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, sc.db, emailAddr, offboardingCredential(firstName, "123"))
	person := testpkg.CreateTestPersonWithAccountID(t, sc.db, firstName, lastName, account.ID)
	staff := testpkg.CreateTestStaffForPerson(t, sc.db, person.ID)
	testpkg.MapAccountToTenant(t, sc.db, account.ID, testpkg.Tenant(t))
	return emailAddr, person.ID, staff.ID
}

// retireMembership arranges a membership retired over a fixed period.
func retireMembership(t *testing.T, db *bun.DB, staffID int64, startedOn, endedOn string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`UPDATE users.staff_school_memberships SET deleted_at = (?::date + time '12:00') AT TIME ZONE 'Europe/Berlin' WHERE id = ?`, endedOn, staffID)
	require.NoError(t, err)
	setMembershipPeriod(t, db, staffID, startedOn, endedOn)
}

// setMembershipPeriod moves a retired membership's lifetime to fixed dates.
func setMembershipPeriod(t *testing.T, db *bun.DB, staffID int64, startedOn, endedOn string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `
		UPDATE users.staff_school_memberships
		SET created_at = (?::date + time '12:00') AT TIME ZONE 'Europe/Berlin',
		    deleted_at = (?::date + time '12:00') AT TIME ZONE 'Europe/Berlin'
		WHERE id = ?`, startedOn, endedOn, staffID)
	require.NoError(t, err)
}

// Without the school's transaction the question is refused instead of
// reading people and memberships outside the request's RLS scope.
func TestFormerEmployment_RequiresTheSchoolTransaction(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t, testpkg.SetupTestDB(t))
	module, err := api.NewTestIdentityAccess(testpkg.NewDatabase(t, sc.db), slog.Default())
	require.NoError(t, err)

	_, found, err := module.FindFormerEmployment(sc.ctx, "former@example.com")
	require.Error(t, err)
	assert.False(t, found)
}
