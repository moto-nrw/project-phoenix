package behavior_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// identityQueryCounter buckets SELECT statements against the identity-chain
// lookups so tests can assert each stage is loaded at most once per request
// context (#2099). Each matcher pairs the table with the lookup column of the
// identity resolution, so relation hydration that merely reads the same table
// by primary-key list (e.g. substitution staff names) stays out of the buckets.
type identityQueryCounter struct {
	*testpkg.QueryCounter
}

// identityStageMatchers maps a stage to a predicate over the lowercased SQL.
var identityStageMatchers = map[string]func(string) bool{
	"persons": func(q string) bool {
		return strings.Contains(q, "users.persons") && strings.Contains(q, "account_id = ")
	},
	"staff": func(q string) bool {
		return strings.Contains(q, "users.staff") && strings.Contains(q, "person_id = ")
	},
	"teachers": func(q string) bool {
		return strings.Contains(q, "users.teachers") && strings.Contains(q, "staff_id = ")
	},
	// Two SQL shapes exist: the hand-written finder emits
	// `.substitute_staff_id = ?`, the Filter-based one `"substitute_staff_id" = ?`.
	"substitutions": func(q string) bool {
		hasTable := strings.Contains(q, "education.group_substitution") ||
			strings.Contains(q, `"education"."group_substitution"`)
		return hasTable &&
			(strings.Contains(q, "substitute_staff_id = ") || strings.Contains(q, `substitute_staff_id" = `))
	},
}

// bucket returns the SELECTs of one identity stage.
func (c *identityQueryCounter) bucket(name string) []string {
	return c.Matching(func(q string) bool {
		return strings.HasPrefix(strings.TrimSpace(q), "select") && identityStageMatchers[name](q)
	})
}

func (c *identityQueryCounter) count(name string) int { return len(c.bucket(name)) }

func newIdentityQueryCounter(t *testing.T, db *bun.DB) *identityQueryCounter {
	t.Helper()
	return &identityQueryCounter{QueryCounter: testpkg.CaptureQueries(t, db)}
}

// resolveFullChain exercises every identity-chain read the way a busy handler
// does: access check, teacher resolution, groups, substitutions, and the
// composite student access decision (which itself nests the staff lookup).
func resolveFullChain(t *testing.T, ctx context.Context, rows *repositories.CallerRows) {
	t.Helper()
	caller := rows.Caller()

	_, err := caller.StaffID(ctx)
	require.NoError(t, err)
	_, err = caller.TeacherID(ctx)
	require.NoError(t, err)
	_, err = rows.GetMyGroups(ctx)
	require.NoError(t, err)
	_, err = rows.GetSubstitutedGroupIDs(ctx)
	require.NoError(t, err)

	access := caller.StudentAccess(ctx)
	require.True(t, access.HasFullAccess())
}

// TestIdentityRequestCacheDedupesChain is the #2099 acceptance test at
// capability level: with the request cache attached, every stage of the
// identity chain is loaded from the database at most once, no matter how
// many chain methods run.
func TestIdentityRequestCacheDedupesChain(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)

	rows := setupCallerRows(t, db)
	today := testpkg.TodayDate()

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "IdentityMemo", "Teacher")
	group := testpkg.CreateTestEducationGroup(t, db, "IdentityMemoGroup")
	_ = testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)
	subGroup := testpkg.CreateTestEducationGroup(t, db, "IdentityMemoSubGroup")
	_ = testpkg.CreateTestGroupSubstitution(t, db, subGroup.ID, nil, teacher.StaffID,
		today.AddDays(-1), today.AddDays(3))

	counter := newIdentityQueryCounter(t, db)

	ctx := jwt.WithRequestIdentityCache(callerCtx(t, account.ID))
	resolveFullChain(t, ctx, rows)

	// Each stage must be loaded exactly once per request. MyGroupIDs and
	// SubstitutedGroupIDs each read substitutions and are memoized at their
	// own result level, so 2 is the per-request floor. Sharing one read
	// between them is out of scope.
	testpkg.AssertQueryBudget(t, "services.usercontext.identity_chain.persons", counter.bucket("persons"))
	testpkg.AssertQueryBudget(t, "services.usercontext.identity_chain.staff", counter.bucket("staff"))
	testpkg.AssertQueryBudget(t, "services.usercontext.identity_chain.teachers", counter.bucket("teachers"))
	testpkg.AssertQueryBudget(t, "services.usercontext.identity_chain.substitutions", counter.bucket("substitutions"))

	// A second full chain on the same context must be entirely memo-served.
	counter.Reset()
	resolveFullChain(t, ctx, rows)
	for _, bucket := range []string{"persons", "staff", "teachers", "substitutions"} {
		assert.Zero(t, counter.count(bucket), "repeat resolution of %s must be served from the request cache", bucket)
	}
}

// TestIdentityWithoutCacheStillQueries proves there is no hidden global state:
// on a plain context (scheduler, CLI, tests) every call keeps hitting the
// database exactly as before #2099.
func TestIdentityWithoutCacheStillQueries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)

	rows := setupCallerRows(t, db)

	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "IdentityNoCache", "Teacher")

	counter := newIdentityQueryCounter(t, db)

	resolveFullChain(t, callerCtx(t, account.ID), rows)

	assert.Greater(t, counter.count("persons"), 1,
		"without the request cache the person stage is resolved per call — memoization must be opt-in via context")
}

// TestNonTeacherStaffNotFoundIsMemoized pins the negative-caching behavior:
// "staff without a teacher role" is a clean outcome and must not re-query
// users.teachers on every MyGroupIDs/TeacherID call.
func TestNonTeacherStaffNotFoundIsMemoized(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)

	rows := setupCallerRows(t, db)

	_, account := testpkg.CreateTestStaffWithAccount(t, db, "IdentityNonTeacher", "Staff")

	counter := newIdentityQueryCounter(t, db)

	ctx := jwt.WithRequestIdentityCache(callerCtx(t, account.ID))

	_, err := rows.GetMyGroups(ctx)
	require.NoError(t, err)
	_, err = rows.GetMyGroups(ctx)
	require.NoError(t, err)
	_, err = rows.Caller().TeacherID(ctx)
	require.Error(t, err, "staff without teacher role keeps returning ErrCallerNotLinkedToTeacher")

	assert.Equal(t, 1, counter.count("teachers"),
		"the clean teacher-not-found outcome must be memoized, not re-queried")
}

// TestUpdateCurrentProfileEvictsIdentity covers the create-person branch: the
// account has no person, the person stage is memoized as "not linked", then
// the profile update creates a person. Without eviction the trailing re-read
// would serve the memoized nil and return an empty name.
func TestUpdateCurrentProfileEvictsIdentity(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)

	caller := setupCallerRows(t, db).Caller()

	email := fmt.Sprintf("identity-evict-%d@test.moto-nrw.de", time.Now().UnixNano())
	account := testpkg.CreateTestAccount(t, db, email)

	ctx := jwt.WithRequestIdentityCache(callerCtx(t, account.ID))

	// Prime the memoized "not linked" person stage.
	_, err := caller.Person(ctx)
	require.Error(t, err)

	first, last := "Frisch", "Angelegt"
	profile, err := caller.UpdateProfile(ctx, identityaccess.CallerProfileUpdate{FirstName: &first, LastName: &last})
	require.NoError(t, err)
	require.NotNil(t, profile.Person,
		"the post-write re-read must see the freshly created person, not the memoized 'not linked' outcome")
	assert.Equal(t, "Frisch", profile.Person.FirstName,
		"the post-write re-read must see the freshly created person, not the memoized 'not linked' outcome")

	_, err = caller.Person(ctx)
	require.NoError(t, err)
}

// TestUpdateAvatarEvictsIdentity covers the account stage: a memoized
// account would ship the old avatar URL in the response.
func TestUpdateAvatarEvictsIdentity(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)

	caller := setupCallerRows(t, db).Caller()

	_, account := testpkg.CreateTestPersonWithAccount(t, db, "IdentityAvatar", "Test")

	ctx := jwt.WithRequestIdentityCache(callerCtx(t, account.ID))

	// Prime the memoized account stage.
	_, err := caller.Account(ctx)
	require.NoError(t, err)

	profile, err := caller.UpdateAvatar(ctx, "/uploads/avatars/identity-evict-test.png")
	require.NoError(t, err)
	assert.Equal(t, "/uploads/avatars/identity-evict-test.png", profile.Account.Avatar,
		"the post-write re-read must see the new avatar, not the memoized account")
}
