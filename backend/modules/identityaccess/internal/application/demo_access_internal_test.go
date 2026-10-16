package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type demoRedemptionStore struct {
	ports.DemoAccessStore
	access domain.DemoAccess
	calls  *[]string
}

func (s demoRedemptionStore) FindDemoAccessByTokenHash(context.Context, string) (domain.DemoAccess, bool, error) {
	*s.calls = append(*s.calls, "find")
	return s.access, true, nil
}

func (s demoRedemptionStore) LockDemoSchool(_ context.Context, slug string) error {
	*s.calls = append(*s.calls, "lock "+slug)
	return nil
}

func (s demoRedemptionStore) RecordDemoAccessUse(_ context.Context, id, accountID, parentAccountID int64, _, _ time.Time) error {
	*s.calls = append(*s.calls, "record")
	return nil
}

type demoRedemptionSchools struct {
	ports.DemoSchools
	calls *[]string
	entry domain.DemoSchoolEntry
}

func (s demoRedemptionSchools) DemoSchoolEntry(context.Context, string) (domain.DemoSchoolEntry, error) {
	*s.calls = append(*s.calls, "entry")
	return s.entry, nil
}

func (s demoRedemptionSchools) MarkDemoSchoolUsed(context.Context, string, time.Time) error {
	*s.calls = append(*s.calls, "used")
	return nil
}

type demoRedemptionSessions struct{ ports.DemoSessions }

func (demoRedemptionSessions) IssueTokensForAuthenticatedAccount(context.Context, int64, int64, string, string) (string, string, error) {
	return "access", "refresh", nil
}

type demoRedemptionTokens struct{ ports.DemoAccessTokens }

func (demoRedemptionTokens) Fingerprint(token string) string { return token }

type demoRedemptionMail struct{ ports.DemoAccessMail }

func TestDemoAccessRedeemLocksTheSchoolBeforeItUsesTheAccess(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	var calls []string
	flow, err := NewDemoAccess(DemoAccessDependencies{
		Sessions: demoRedemptionSessions{},
		Store: demoRedemptionStore{
			access: domain.DemoAccess{ID: 7, ExpiresAt: now.Add(time.Hour), SchoolSlug: "ogs-nord-k3m9xp"},
			calls:  &calls,
		},
		Schools: demoRedemptionSchools{
			calls: &calls,
			entry: domain.DemoSchoolEntry{Exists: true, Status: domain.DemoSchoolReady, TenantID: 3, AccountID: 5},
		},
		Tokens: demoRedemptionTokens{},
		Mail:   demoRedemptionMail{},
		AdminTx: func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	})
	require.NoError(t, err)
	flow.now = func() time.Time { return now }

	_, err = flow.Redeem(t.Context(), "token", "", "127.0.0.1", "test")
	require.NoError(t, err)
	assert.Equal(t, []string{"find", "lock ogs-nord-k3m9xp", "find", "entry", "used", "record"}, calls)
}

type movingDemoAccessStore struct {
	ports.DemoAccessStore
	accesses []domain.DemoAccess
	locks    []string
	reads    int
}

func (s *movingDemoAccessStore) FindDemoAccessByTokenHash(context.Context, string) (domain.DemoAccess, bool, error) {
	access := s.accesses[s.reads]
	s.reads++
	return access, true, nil
}

func (s *movingDemoAccessStore) LockDemoSchool(_ context.Context, slug string) error {
	s.locks = append(s.locks, slug)
	return nil
}

func (s *movingDemoAccessStore) FindActiveDemoAccessByEmail(context.Context, string, time.Time) (domain.DemoAccess, bool, error) {
	access := s.accesses[s.reads]
	s.reads++
	return access, true, nil
}

func TestDemoAccessLocksTheReplacementSchoolAfterAConcurrentReset(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	store := &movingDemoAccessStore{accesses: []domain.DemoAccess{
		{ExpiresAt: now.Add(time.Hour), SchoolSlug: "ogs-nord-k3m9xp"},
		{ExpiresAt: now.Add(time.Hour), SchoolSlug: "ogs-sued-b8n2lt"},
		{ExpiresAt: now.Add(time.Hour), SchoolSlug: "ogs-sued-b8n2lt"},
		{ExpiresAt: now.Add(time.Hour), SchoolSlug: "ogs-sued-b8n2lt"},
	}}
	flow := &DemoAccess{store: store, tokens: demoRedemptionTokens{}, now: func() time.Time { return now }}

	access, err := flow.validAndLock(t.Context(), "token")
	require.NoError(t, err)
	assert.Equal(t, "ogs-sued-b8n2lt", access.SchoolSlug)
	assert.Equal(t, []string{"ogs-nord-k3m9xp", "ogs-sued-b8n2lt"}, store.locks)

	store.reads = 0
	store.locks = nil
	access, found, coolingDown, err := flow.knownSchool(t.Context(), "besuch@example.org")
	require.NoError(t, err)
	assert.True(t, found)
	assert.False(t, coolingDown)
	assert.Equal(t, "ogs-sued-b8n2lt", access.SchoolSlug)
	assert.Equal(t, []string{"ogs-nord-k3m9xp", "ogs-sued-b8n2lt"}, store.locks)
}
