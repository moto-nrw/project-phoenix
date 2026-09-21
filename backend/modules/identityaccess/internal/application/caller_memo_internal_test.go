package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// memoCaller is the authenticated caller of the memo tests: account 42 in
// tenant 7.
func memoCaller() domain.Caller {
	return domain.Caller{Authenticated: true, AccountID: 42, TenantID: 7, ClaimsTenantID: 7}
}

func memoHarness() *callerHarness {
	h := newCallerHarness(memoCaller())
	h.memo = newCallerTestMemo()
	return h
}

func TestCallerEntryRequiresMemoAndAccount(t *testing.T) {
	t.Parallel()

	withoutMemo := newCallerHarness(memoCaller())
	assert.Nil(t, withoutMemo.build(t).entry(context.Background()), "no memo attached")

	anonymous := newCallerHarness(domain.Caller{})
	anonymous.memo = newCallerTestMemo()
	assert.Nil(t, anonymous.build(t).entry(context.Background()), "memo attached but no authenticated account")
	assert.Empty(t, anonymous.memo.keys, "an anonymous caller must not touch the memo")

	h := memoHarness()
	entry := h.build(t).entry(context.Background())
	require.NotNil(t, entry)
	assert.Equal(t, []callerTestMemoKey{{tenantID: 7, accountID: 42}}, h.memo.keys)
}

func TestCallerEntryStagesMissStoreHit(t *testing.T) {
	t.Parallel()

	entry := memoHarness().build(t).entry(context.Background())
	require.NotNil(t, entry)

	_, hit := entry.cachedPerson()
	assert.False(t, hit, "empty entry must miss")

	person := &domain.CallerPerson{FirstName: "Memo", LastName: "Test"}
	entry.storePerson(person)
	got, hit := entry.cachedPerson()
	require.True(t, hit)
	assert.Equal(t, person, got)

	// A loaded stage with a zero value is a clean "not linked" outcome - a hit.
	entry.storePerson(nil)
	nilPerson, hit := entry.cachedPerson()
	require.True(t, hit)
	assert.Nil(t, nilPerson)

	entry.storeStaff(0)
	staff, hit := entry.cachedStaff()
	require.True(t, hit)
	assert.Zero(t, staff)

	entry.storeTeacher(0)
	teacher, hit := entry.cachedTeacher()
	require.True(t, hit)
	assert.Zero(t, teacher)
}

func TestCallerEntryIsolatesTenantsAndAccounts(t *testing.T) {
	t.Parallel()

	h := memoHarness()
	c := h.build(t)
	ctx := context.Background()
	c.entry(ctx).storePerson(&domain.CallerPerson{FirstName: "Tenant7"})

	// Same memo, different tenant on the caller: distinct entry.
	h.caller.TenantID = 8
	_, hit := c.entry(ctx).cachedPerson()
	assert.False(t, hit, "another tenant's entry must not be served")

	// Different account, same tenant.
	h.caller.TenantID, h.caller.AccountID = 7, 43
	_, hit = c.entry(ctx).cachedPerson()
	assert.False(t, hit, "another account's entry must not be served")

	h.caller.AccountID = 42
	_, hit = c.entry(ctx).cachedPerson()
	assert.True(t, hit, "the original entry is still served")
}

func TestCallerInvalidateDropsAllStages(t *testing.T) {
	t.Parallel()

	c := memoHarness().build(t)
	ctx := context.Background()
	entry := c.entry(ctx)

	entry.storePerson(&domain.CallerPerson{})
	entry.storeStaff(testStaffID)
	entry.storeGroups([]int64{11})
	entry.storeSubstitutions(map[int64]bool{5: true})

	c.invalidate(ctx)

	fresh := c.entry(ctx)
	for name, probe := range map[string]func() bool{
		"person": func() bool { _, hit := fresh.cachedPerson(); return hit },
		"staff":  func() bool { _, hit := fresh.cachedStaff(); return hit },
		"groups": func() bool { _, hit := fresh.cachedGroups(); return hit },
		"subs":   func() bool { _, hit := fresh.cachedSubstitutions(); return hit },
	} {
		assert.False(t, probe(), "stage %s must be dropped after invalidate", name)
	}
}

// Consumers sort the MyGroupIDs result in place: a caller-visible mutation
// must never reorder or poison the memoized value.
func TestCallerEntryGroupsAndSubsAreDefensivelyCopied(t *testing.T) {
	t.Parallel()

	entry := memoHarness().build(t).entry(context.Background())
	require.NotNil(t, entry)

	stored := []int64{22, 11}
	entry.storeGroups(stored)
	// Mutating the slice we stored must not affect the memo either.
	stored[0], stored[1] = stored[1], stored[0]

	first, hit := entry.cachedGroups()
	require.True(t, hit)
	require.Len(t, first, 2)
	assert.Equal(t, int64(22), first[0])

	// Mutate the returned slice; a re-read must be unaffected.
	first[0], first[1] = first[1], first[0]
	second, hit := entry.cachedGroups()
	require.True(t, hit)
	assert.Equal(t, int64(22), second[0])

	subs := map[int64]bool{5: true}
	entry.storeSubstitutions(subs)
	subs[6] = true // mutate the stored-from map
	got, hit := entry.cachedSubstitutions()
	require.True(t, hit)
	assert.Equal(t, map[int64]bool{5: true}, got)

	got[7] = true // mutate the returned map
	again, hit := entry.cachedSubstitutions()
	require.True(t, hit)
	assert.Equal(t, map[int64]bool{5: true}, again)
}

func TestIsAuthenticated(t *testing.T) {
	t.Parallel()

	accountID, err := newCallerHarness(authenticatedCaller(true)).build(t).accountID(context.Background())
	require.NoError(t, err)
	assert.Equal(t, testAccountID, accountID)

	_, err = newCallerHarness(domain.Caller{}).build(t).accountID(context.Background())
	require.ErrorIs(t, err, domain.ErrCallerNotAuthenticated)
}

func TestMyGroupIDs_RejectsUnauthenticated(t *testing.T) {
	t.Parallel()

	groups, err := newCallerHarness(domain.Caller{}).build(t).MyGroupIDs(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrCallerNotAuthenticated)
	assert.Nil(t, groups)
}

func TestCallerContextLogger_FallsBackToDefault(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, newCallerHarness(domain.Caller{}).build(t).logger())
}
