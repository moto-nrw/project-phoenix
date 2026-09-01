package usercontext_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usercontextSvc "github.com/moto-nrw/project-phoenix/services/usercontext"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// identityQueryCounter buckets SELECT statements against the identity-chain
// lookups so tests can assert each stage is loaded at most once per request
// context (#2099). Each matcher pairs the table with the lookup column of the
// identity resolution, so relation hydration that merely reads the same table
// by primary-key list (e.g. substitution staff names) stays out of the buckets.
type identityQueryCounter struct {
	mu      sync.Mutex
	buckets map[string]int
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

func (c *identityQueryCounter) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	query := strings.ToLower(event.Query)
	if !strings.HasPrefix(strings.TrimSpace(query), "select") {
		return ctx
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for bucket, matches := range identityStageMatchers {
		if matches(query) {
			c.buckets[bucket]++
		}
	}
	return ctx
}

func (*identityQueryCounter) AfterQuery(context.Context, *bun.QueryEvent) {}

func (c *identityQueryCounter) count(bucket string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buckets[bucket]
}

func (c *identityQueryCounter) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buckets = make(map[string]int)
}

func newIdentityQueryCounter(db *bun.DB) *identityQueryCounter {
	counter := &identityQueryCounter{buckets: make(map[string]int)}
	db.AddQueryHook(counter)
	return counter
}

// resolveFullChain exercises every identity-chain read the way a busy handler
// does: access check, teacher resolution, groups, substitutions, and the
// composite ResolveStudentAccess (which itself nests the staff lookup).
func resolveFullChain(t *testing.T, ctx context.Context, service usercontextSvc.UserContextService) {
	t.Helper()

	_, err := service.GetCurrentStaff(ctx)
	require.NoError(t, err)
	_, err = service.GetCurrentTeacher(ctx)
	require.NoError(t, err)
	_, err = service.GetMyGroups(ctx)
	require.NoError(t, err)
	_, err = service.GetSubstitutedGroupIDs(ctx)
	require.NoError(t, err)

	access := usercontextSvc.ResolveStudentAccess(ctx, service)
	require.NotNil(t, access)
}

// TestIdentityRequestCacheDedupesChain is the #2099 acceptance test at service
// level: with the request cache attached, every stage of the identity chain is
// loaded from the database at most once, no matter how many chain methods run.
func TestIdentityRequestCacheDedupesChain(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupIsolatedTestDB(t)

	service := setupUserContextService(t, db)
	today := timezone.TodayDate()

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "IdentityMemo", "Teacher")
	group := testpkg.CreateTestEducationGroup(t, db, "IdentityMemoGroup")
	_ = testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)
	subGroup := testpkg.CreateTestEducationGroup(t, db, "IdentityMemoSubGroup")
	_ = testpkg.CreateTestGroupSubstitution(t, db, subGroup.ID, nil, teacher.StaffID,
		today.AddDays(-1), today.AddDays(3))

	counter := newIdentityQueryCounter(db)

	ctx := usercontextSvc.WithIdentityRequestCache(contextWithClaims(t, int(account.ID)))
	resolveFullChain(t, ctx, service)

	assert.Equal(t, 1, counter.count("persons"), "users.persons must be loaded exactly once per request")
	assert.Equal(t, 1, counter.count("staff"), "users.staff must be loaded exactly once per request")
	assert.Equal(t, 1, counter.count("teachers"), "users.teachers must be loaded exactly once per request")
	// Two distinct repo projections read substitutions (GetMyGroups uses
	// FindActiveBySubstituteWithRelations, GetSubstitutedGroupIDs uses
	// FindActiveBySubstitute); each is memoized at its result level, so 2 is
	// the per-request floor. Unifying the two projections is out of scope.
	assert.Equal(t, 2, counter.count("substitutions"), "substitution lookups must run once per projection")

	// A second full chain on the same context must be entirely memo-served.
	counter.reset()
	resolveFullChain(t, ctx, service)
	for _, bucket := range []string{"persons", "staff", "teachers", "substitutions"} {
		assert.Zero(t, counter.count(bucket), "repeat resolution of %s must be served from the request cache", bucket)
	}
}

// TestIdentityWithoutCacheStillQueries proves there is no hidden global state:
// on a plain context (scheduler, CLI, tests) every call keeps hitting the
// database exactly as before #2099.
func TestIdentityWithoutCacheStillQueries(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupIsolatedTestDB(t)

	service := setupUserContextService(t, db)

	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "IdentityNoCache", "Teacher")

	counter := newIdentityQueryCounter(db)

	ctx := contextWithClaims(t, int(account.ID))
	resolveFullChain(t, ctx, service)

	assert.Greater(t, counter.count("persons"), 1,
		"without the request cache the person stage is resolved per call — memoization must be opt-in via context")
}

// TestNonTeacherStaffNotFoundIsMemoized pins the negative-caching behavior:
// "staff without a teacher role" is a clean outcome and must not re-query
// users.teachers on every GetMyGroups/GetCurrentTeacher call.
func TestNonTeacherStaffNotFoundIsMemoized(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupIsolatedTestDB(t)

	service := setupUserContextService(t, db)

	_, account := testpkg.CreateTestStaffWithAccount(t, db, "IdentityNonTeacher", "Staff")

	counter := newIdentityQueryCounter(db)

	ctx := usercontextSvc.WithIdentityRequestCache(contextWithClaims(t, int(account.ID)))

	_, err := service.GetMyGroups(ctx)
	require.NoError(t, err)
	_, err = service.GetMyGroups(ctx)
	require.NoError(t, err)
	_, err = service.GetCurrentTeacher(ctx)
	require.Error(t, err, "staff without teacher role keeps returning ErrUserNotLinkedToTeacher")

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

	service := setupUserContextService(t, db)

	email := fmt.Sprintf("identity-evict-%d@test.moto-nrw.de", time.Now().UnixNano())
	account := testpkg.CreateTestAccount(t, db, email)

	ctx := usercontextSvc.WithIdentityRequestCache(contextWithClaims(t, int(account.ID)))

	// Prime the memoized "not linked" person stage.
	_, err := service.GetCurrentPerson(ctx)
	require.Error(t, err)

	profile, err := service.UpdateCurrentProfile(ctx, map[string]interface{}{
		"first_name": "Frisch",
		"last_name":  "Angelegt",
	})
	require.NoError(t, err)
	assert.Equal(t, "Frisch", profile["first_name"],
		"the post-write re-read must see the freshly created person, not the memoized 'not linked' outcome")

	// The created person must be cleaned up too; resolve it for the deferred cleanup.
	_, err = service.GetCurrentPerson(ctx)
	require.NoError(t, err)
}

// TestUpdateAvatarEvictsIdentity covers the account stage: UpdateAvatar writes
// via the repository without mutating the in-memory struct, so a memoized
// account would ship the old avatar URL in the response.
func TestUpdateAvatarEvictsIdentity(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupIsolatedTestDB(t)

	service := setupUserContextService(t, db)

	_, account := testpkg.CreateTestPersonWithAccount(t, db, "IdentityAvatar", "Test")

	ctx := usercontextSvc.WithIdentityRequestCache(contextWithClaims(t, int(account.ID)))

	// Prime the memoized account stage.
	_, err := service.GetCurrentUser(ctx)
	require.NoError(t, err)

	profile, err := service.UpdateAvatar(ctx, "/uploads/avatars/identity-evict-test.png")
	require.NoError(t, err)
	assert.Equal(t, "/uploads/avatars/identity-evict-test.png", profile["avatar"],
		"the post-write re-read must see the new avatar, not the memoized account")
}
