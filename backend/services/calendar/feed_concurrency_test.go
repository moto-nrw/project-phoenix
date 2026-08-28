package calendar

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type concurrentFeedRepo struct {
	authModels.StaffCalendarFeedTokenRepository
	firstHash     string
	secondEntered chan struct{}
	calls         atomic.Int32
	mu            sync.Mutex
}

func (r *concurrentFeedRepo) EnsureToken(_ context.Context, _, _ int64, tokenHash string) (string, error) {
	call := r.calls.Add(1)
	r.mu.Lock()
	if r.firstHash == "" {
		r.firstHash = tokenHash
	}
	firstHash := r.firstHash
	if call == 2 {
		close(r.secondEntered)
	}
	r.mu.Unlock()

	if call == 1 {
		select {
		case <-r.secondEntered:
		case <-time.After(250 * time.Millisecond):
		}
	}
	return firstHash, nil
}

type feedUserContext struct {
	usercontext.UserContextService
	account *authModels.Account
	staff   *userModels.Staff
}

func (u *feedUserContext) GetCurrentUser(context.Context) (*authModels.Account, error) {
	return u.account, nil
}

func (u *feedUserContext) GetCurrentStaff(context.Context) (*userModels.Staff, error) {
	return u.staff, nil
}

func TestStaffCalendarFeedURLSharesConcurrentFirstToken(t *testing.T) {
	t.Parallel()

	const (
		accountID = int64(41)
		tenantID  = int64(73)
	)
	repo := &concurrentFeedRepo{secondEntered: make(chan struct{})}
	service := NewService(Config{
		StaffFeedRepo: repo,
		UserContext: &feedUserContext{
			account: &authModels.Account{Model: base.Model{ID: accountID}, Active: true},
			staff:   &userModels.Staff{Model: base.Model{ID: 91}},
		},
		FrontendURL: "https://moto.test",
	})
	ctx := tenant.WithTenantID(context.Background(), tenantID)
	ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: int(accountID), TenantID: tenantID})

	type result struct {
		httpsURL  string
		webcalURL string
		err       error
	}
	start := make(chan struct{})
	ready := sync.WaitGroup{}
	ready.Add(2)
	results := make(chan result, 2)
	for range 2 {
		go func() {
			ready.Done()
			<-start
			httpsURL, webcalURL, err := service.StaffCalendarFeedURL(ctx)
			results <- result{httpsURL: httpsURL, webcalURL: webcalURL, err: err}
		}()
	}
	ready.Wait()
	close(start)

	first := <-results
	second := <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	assert.NotEmpty(t, first.httpsURL)
	assert.Equal(t, first.httpsURL, second.httpsURL)
	assert.Equal(t, first.webcalURL, second.webcalURL)
	assert.Equal(t, int32(1), repo.calls.Load())
}
