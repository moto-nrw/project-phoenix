package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/email"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	platformModel "github.com/moto-nrw/project-phoenix/models/platform"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testRateLimitRepo provides an in-memory implementation of the password reset rate limiter.
type testRateLimitRepo struct {
	mu          sync.Mutex
	attempts    int
	windowStart time.Time
}

func newTestRateLimitRepo() *testRateLimitRepo {
	return &testRateLimitRepo{}
}

func (r *testRateLimitRepo) CheckRateLimit(_ context.Context, _ string) (*authModel.RateLimitState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	retryAt := r.windowStart
	if retryAt.IsZero() {
		retryAt = time.Now()
	}

	return &authModel.RateLimitState{
		Attempts: r.attempts,
		RetryAt:  retryAt.Add(time.Hour),
	}, nil
}

func (r *testRateLimitRepo) IncrementAttempts(_ context.Context, _ string) (*authModel.RateLimitState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if r.windowStart.IsZero() || now.Sub(r.windowStart) > time.Hour {
		r.windowStart = now
		r.attempts = 1
	} else {
		r.attempts++
	}

	return &authModel.RateLimitState{
		Attempts: r.attempts,
		RetryAt:  r.windowStart.Add(time.Hour),
	}, nil
}

func (r *testRateLimitRepo) CleanupExpired(_ context.Context) (int, error) {
	return 0, nil
}

func (r *testRateLimitRepo) setWindow(start time.Time, attempts int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.windowStart = start
	r.attempts = attempts
}

func (r *testRateLimitRepo) Attempts() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.attempts
}

func (r *testRateLimitRepo) RetryAt() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.windowStart.IsZero() {
		return time.Time{}
	}
	return r.windowStart.Add(time.Hour)
}

// flakyMailer fails a configurable number of initial attempts before succeeding.
type flakyMailer struct {
	mu        sync.Mutex
	failCount int
	err       error
	attempts  int
	messages  []email.Message
}

func newFlakyMailer(failures int, err error) *flakyMailer {
	if failures < 0 {
		failures = 0
	}
	if err == nil {
		err = errors.New("mailer failure")
	}
	return &flakyMailer{failCount: failures, err: err}
}

func (m *flakyMailer) Send(msg email.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attempts++
	if m.attempts <= m.failCount {
		return m.err
	}
	m.messages = append(m.messages, msg)
	return nil
}

func (m *flakyMailer) Attempts() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.attempts
}

func (m *flakyMailer) Messages() []email.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]email.Message, len(m.messages))
	copy(out, m.messages)
	return out
}

// noopAccountRepository provides default panic implementations for unused methods.
type noopAccountRepository struct{}

func (noopAccountRepository) Create(context.Context, *authModel.Account) error {
	panic("Create not implemented")
}

func (noopAccountRepository) FindByID(context.Context, interface{}) (*authModel.Account, error) {
	panic("FindByID not implemented")
}

func (noopAccountRepository) FindManageableByID(context.Context, int64) (*authModel.Account, error) {
	panic("FindManageableByID not implemented")
}

func (noopAccountRepository) ListManageable(context.Context, map[string]interface{}) ([]*authModel.Account, error) {
	panic("ListManageable not implemented")
}

func (noopAccountRepository) UpdateManageable(context.Context, *authModel.Account) error {
	panic("UpdateManageable not implemented")
}

func (noopAccountRepository) FindByIDForUpdate(context.Context, int64) (*authModel.Account, error) {
	panic("FindByIDForUpdate not implemented")
}

func (noopAccountRepository) FindByEmail(context.Context, string) (*authModel.Account, error) {
	panic("FindByEmail not implemented")
}

func (noopAccountRepository) FindByUsername(context.Context, string) (*authModel.Account, error) {
	panic("FindByUsername not implemented")
}

func (noopAccountRepository) FindByCalendarFeedToken(context.Context, string) (*authModel.Account, error) {
	panic("FindByCalendarFeedToken not implemented")
}

func (noopAccountRepository) SetCalendarFeedToken(context.Context, int64, string) error {
	panic("SetCalendarFeedToken not implemented")
}

func (noopAccountRepository) EnsureCalendarFeedToken(context.Context, int64, string) (string, error) {
	panic("EnsureCalendarFeedToken not implemented")
}

func (noopAccountRepository) Update(context.Context, *authModel.Account) error {
	panic("Update not implemented")
}

func (noopAccountRepository) Delete(context.Context, interface{}) error {
	panic("Delete not implemented")
}

func (noopAccountRepository) List(context.Context, map[string]interface{}) ([]*authModel.Account, error) {
	panic("List not implemented")
}

func (noopAccountRepository) UpdateLastLogin(context.Context, int64) error {
	panic("UpdateLastLogin not implemented")
}

func (noopAccountRepository) UpdatePassword(context.Context, int64, string) error {
	panic("UpdatePassword not implemented")
}

func (noopAccountRepository) SetActive(context.Context, int64, bool) error {
	panic("SetActive not implemented")
}

func (noopAccountRepository) UpdateAvatar(context.Context, int64, string) error {
	panic("UpdateAvatar not implemented")
}

func (noopAccountRepository) FindByRole(context.Context, string) ([]*authModel.Account, error) {
	panic("FindByRole not implemented")
}

func (noopAccountRepository) ListEffectiveAdminAccountIDs(context.Context) ([]int64, error) {
	panic("ListEffectiveAdminAccountIDs not implemented")
}

func (noopAccountRepository) FindAccountsWithRolesAndPermissions(context.Context, map[string]interface{}) ([]*authModel.Account, error) {
	panic("FindAccountsWithRolesAndPermissions not implemented")
}

func (noopAccountRepository) FindEmailsByAccountIDs(context.Context, []int64) (map[int64]string, error) {
	panic("FindEmailsByAccountIDs not implemented")
}

func (noopAccountRepository) IncrementMFAAttempts(context.Context, int64, int, time.Duration) (authModel.MFAAttemptResult, error) {
	panic("IncrementMFAAttempts not implemented")
}

func (noopAccountRepository) ResetMFAAttempts(context.Context, int64) error {
	panic("ResetMFAAttempts not implemented")
}

func (noopAccountRepository) IncrementPINAttempts(context.Context, int64, int, time.Duration) (authModel.PINAttemptResult, error) {
	panic("IncrementPINAttempts not implemented")
}

func (noopAccountRepository) ResetPINAttempts(context.Context, int64) error {
	panic("ResetPINAttempts not implemented")
}

func (noopAccountRepository) ClearPIN(context.Context, int64) error {
	panic("ClearPIN not implemented")
}

func (noopAccountRepository) FindAvatarsByAccountIDs(context.Context, []int64) (map[int64]string, error) {
	panic("FindAvatarsByAccountIDs not implemented")
}

// stubAccountRepository implements a minimal in-memory account store.
type stubAccountRepository struct {
	noopAccountRepository

	mu       sync.Mutex
	accounts map[string]*authModel.Account
	byID     map[int64]*authModel.Account
	nextID   int64

	failCreate bool
}

func newStubAccountRepository(initial ...*authModel.Account) *stubAccountRepository {
	repo := &stubAccountRepository{
		accounts: make(map[string]*authModel.Account),
		byID:     make(map[int64]*authModel.Account),
		nextID:   0,
	}
	for _, acc := range initial {
		repo.storeAccount(acc)
	}
	return repo
}

func (r *stubAccountRepository) storeAccount(acc *authModel.Account) {
	if acc.ID == 0 {
		r.nextID++
		acc.ID = r.nextID
	} else if acc.ID > r.nextID {
		r.nextID = acc.ID
	}
	emailKey := strings.ToLower(acc.Email)
	r.accounts[emailKey] = acc
	r.byID[acc.ID] = acc
}

func (r *stubAccountRepository) Create(_ context.Context, account *authModel.Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failCreate {
		return errors.New("account create failed")
	}
	r.storeAccount(account)
	return nil
}

// FindByEmail returns a copy. Real DB repo loads a fresh struct per call;
// returning the stored pointer here would let callers see in-memory mutations
// (e.g. UpdatePassword) that production code can never observe.
func (r *stubAccountRepository) FindByEmail(_ context.Context, email string) (*authModel.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if acc, ok := r.accounts[strings.ToLower(email)]; ok {
		clone := *acc
		return &clone, nil
	}
	return nil, sql.ErrNoRows
}

func (r *stubAccountRepository) FindByCalendarFeedToken(context.Context, string) (*authModel.Account, error) {
	return nil, sql.ErrNoRows
}

func (r *stubAccountRepository) EnsureCalendarFeedToken(context.Context, int64, string) (string, error) {
	return "", nil
}

func (r *stubAccountRepository) SetCalendarFeedToken(context.Context, int64, string) error {
	return nil
}

func (r *stubAccountRepository) FindByID(_ context.Context, id interface{}) (*authModel.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v, ok := id.(int64); ok {
		if acc, exists := r.byID[v]; exists {
			clone := *acc
			return &clone, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *stubAccountRepository) FindManageableByID(ctx context.Context, id int64) (*authModel.Account, error) {
	return r.FindByID(ctx, id)
}

func (r *stubAccountRepository) UpdatePassword(_ context.Context, id int64, hash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if acc, ok := r.byID[id]; ok {
		acc.PasswordHash = &hash
		return nil
	}
	return sql.ErrNoRows
}

func (r *stubAccountRepository) Update(_ context.Context, account *authModel.Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if stored, ok := r.byID[account.ID]; ok {
		stored.Active = account.Active
		stored.Email = account.Email
		stored.Username = account.Username
		stored.PasswordHash = account.PasswordHash
		stored.Avatar = account.Avatar
		return nil
	}
	return sql.ErrNoRows
}

func (r *stubAccountRepository) UpdateManageable(ctx context.Context, account *authModel.Account) error {
	return r.Update(ctx, account)
}

func (r *stubAccountRepository) UpdateAvatar(_ context.Context, id int64, avatar string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if acc, ok := r.byID[id]; ok {
		acc.Avatar = avatar
		return nil
	}
	return sql.ErrNoRows
}

func (r *stubAccountRepository) SetActive(_ context.Context, id int64, active bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if acc, ok := r.byID[id]; ok {
		acc.Active = active
		return nil
	}
	return sql.ErrNoRows
}

// IncrementMFAAttempts / ResetMFAAttempts are part of the
// AccountRepository interface as of #1430 review item #6 (atomic MFA
// lockout counter). This stub doesn't exercise the MFA flow, so the
// methods panic if any test in this package ever reaches them — that's
// a signal to wire a richer fake rather than silently no-op.
func (r *stubAccountRepository) IncrementMFAAttempts(_ context.Context, _ int64, _ int, _ time.Duration) (authModel.MFAAttemptResult, error) {
	panic("IncrementMFAAttempts not implemented in stubAccountRepository")
}

func (r *stubAccountRepository) ResetMFAAttempts(_ context.Context, _ int64) error {
	panic("ResetMFAAttempts not implemented in stubAccountRepository")
}

func (r *stubAccountRepository) IncrementPINAttempts(_ context.Context, _ int64, _ int, _ time.Duration) (authModel.PINAttemptResult, error) {
	panic("IncrementPINAttempts not implemented in stubAccountRepository")
}

func (r *stubAccountRepository) ResetPINAttempts(_ context.Context, _ int64) error {
	panic("ResetPINAttempts not implemented in stubAccountRepository")
}

func (r *stubAccountRepository) ClearPIN(_ context.Context, _ int64) error {
	panic("ClearPIN not implemented in stubAccountRepository")
}

// noopPasswordResetTokenRepository provides default panic implementations.
type noopPasswordResetTokenRepository struct{}

func (noopPasswordResetTokenRepository) Create(context.Context, *authModel.PasswordResetToken) error {
	panic("Create not implemented")
}

func (noopPasswordResetTokenRepository) FindByID(context.Context, interface{}) (*authModel.PasswordResetToken, error) {
	panic("FindByID not implemented")
}

func (noopPasswordResetTokenRepository) Update(context.Context, *authModel.PasswordResetToken) error {
	panic("Update not implemented")
}

func (noopPasswordResetTokenRepository) Delete(context.Context, interface{}) error {
	panic("Delete not implemented")
}

func (noopPasswordResetTokenRepository) UpdateDeliveryResult(context.Context, int64, *time.Time, *string, int) error {
	panic("UpdateDeliveryResult not implemented")
}

func (noopPasswordResetTokenRepository) List(context.Context, map[string]interface{}) ([]*authModel.PasswordResetToken, error) {
	panic("List not implemented")
}

func (noopPasswordResetTokenRepository) FindByToken(context.Context, string) (*authModel.PasswordResetToken, error) {
	panic("FindByToken not implemented")
}

func (noopPasswordResetTokenRepository) FindByAccountID(context.Context, int64) ([]*authModel.PasswordResetToken, error) {
	panic("FindByAccountID not implemented")
}

func (noopPasswordResetTokenRepository) FindValidByToken(context.Context, string) (*authModel.PasswordResetToken, error) {
	panic("FindValidByToken not implemented")
}

func (noopPasswordResetTokenRepository) MarkAsUsed(context.Context, int64) error {
	panic("MarkAsUsed not implemented")
}

func (noopPasswordResetTokenRepository) DeleteExpiredTokens(context.Context) (int, error) {
	panic("DeleteExpiredTokens not implemented")
}

func (noopPasswordResetTokenRepository) InvalidateTokensByAccountID(context.Context, int64) error {
	panic("InvalidateTokensByAccountID not implemented")
}

// stubPasswordResetTokenRepository stores tokens in memory.
type stubPasswordResetTokenRepository struct {
	noopPasswordResetTokenRepository

	mu     sync.Mutex
	tokens map[string]*authModel.PasswordResetToken
	byID   map[int64]*authModel.PasswordResetToken
	nextID int64
}

func newStubPasswordResetTokenRepository() *stubPasswordResetTokenRepository {
	return &stubPasswordResetTokenRepository{
		tokens: make(map[string]*authModel.PasswordResetToken),
		byID:   make(map[int64]*authModel.PasswordResetToken),
	}
}

func (r *stubPasswordResetTokenRepository) Create(_ context.Context, token *authModel.PasswordResetToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if token.ID == 0 {
		r.nextID++
		token.ID = r.nextID
	}
	r.tokens[token.Token] = token
	r.byID[token.ID] = token
	return nil
}

func (r *stubPasswordResetTokenRepository) FindByID(_ context.Context, id interface{}) (*authModel.PasswordResetToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch v := id.(type) {
	case int64:
		if token, ok := r.byID[v]; ok {
			// Copy, like a real repository returns a fresh row: tests poll
			// the result while the async delivery goroutine mutates the
			// stored token under r.mu.
			cp := *token
			return &cp, nil
		}
	case int:
		if token, ok := r.byID[int64(v)]; ok {
			cp := *token
			return &cp, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *stubPasswordResetTokenRepository) FindValidByToken(_ context.Context, token string) (*authModel.PasswordResetToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.tokens[token]
	if !ok || item.Used || time.Now().After(item.Expiry) {
		return nil, sql.ErrNoRows
	}
	cp := *item
	return &cp, nil
}

func (r *stubPasswordResetTokenRepository) MarkAsUsed(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if token, ok := r.byID[id]; ok {
		token.Used = true
		return nil
	}
	return sql.ErrNoRows
}

func (r *stubPasswordResetTokenRepository) UpdateDeliveryResult(_ context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	token, exists := r.byID[tokenID]
	if !exists {
		return sql.ErrNoRows
	}
	token.EmailRetryCount = retryCount
	if sentAt != nil {
		token.EmailSentAt = sentAt
	} else {
		token.EmailSentAt = nil
	}
	if emailError != nil {
		token.EmailError = emailError
	} else {
		token.EmailError = nil
	}
	return nil
}

func (r *stubPasswordResetTokenRepository) InvalidateTokensByAccountID(_ context.Context, accountID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, token := range r.tokens {
		if token.AccountID == accountID {
			token.Used = true
			delete(r.tokens, key)
		}
	}
	return nil
}

// stubInvitationTokenRepository stores invitations in memory.
type stubInvitationTokenRepository struct {
	mu      sync.Mutex
	tokens  map[int64]*authModel.InvitationToken
	byToken map[string]*authModel.InvitationToken
	nextID  int64
	nowFunc func() time.Time
}

const invitationTestTenantID int64 = 1

func newStubInvitationTokenRepository() *stubInvitationTokenRepository {
	return &stubInvitationTokenRepository{
		tokens:  make(map[int64]*authModel.InvitationToken),
		byToken: make(map[string]*authModel.InvitationToken),
		nowFunc: time.Now,
	}
}

func (r *stubInvitationTokenRepository) now() time.Time {
	if r.nowFunc != nil {
		return r.nowFunc()
	}
	return time.Now()
}

func (r *stubInvitationTokenRepository) Create(_ context.Context, token *authModel.InvitationToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if token.ID == 0 {
		r.nextID++
		token.ID = r.nextID
	}
	if token.TenantID == 0 {
		token.TenantID = invitationTestTenantID
	}
	r.tokens[token.ID] = token
	r.byToken[token.Token] = token
	return nil
}

func (r *stubInvitationTokenRepository) Update(_ context.Context, token *authModel.InvitationToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[token.ID] = token
	r.byToken[token.Token] = token
	return nil
}

func (r *stubInvitationTokenRepository) FindByID(_ context.Context, id interface{}) (*authModel.InvitationToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v, ok := id.(int64); ok {
		if token, exists := r.tokens[v]; exists {
			// Return a copy, like a real repository returns a fresh row.
			// Tests poll the returned token while the service's async
			// delivery goroutine mutates the stored one under r.mu;
			// handing out the live pointer is a data race.
			cp := *token
			return &cp, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *stubInvitationTokenRepository) FindByToken(_ context.Context, value string) (*authModel.InvitationToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if token, ok := r.byToken[value]; ok {
		cp := *token
		return &cp, nil
	}
	return nil, sql.ErrNoRows
}

func (r *stubInvitationTokenRepository) FindValidByToken(_ context.Context, value string, now time.Time) (*authModel.InvitationToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	token, ok := r.byToken[value]
	if !ok {
		return nil, sql.ErrNoRows
	}
	if token.IsUsed() || token.ExpiresAt.Before(now) {
		return nil, sql.ErrNoRows
	}
	cp := *token
	return &cp, nil
}

func (r *stubInvitationTokenRepository) FindByEmail(_ context.Context, email string) ([]*authModel.InvitationToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	email = strings.ToLower(email)
	var result []*authModel.InvitationToken
	for _, token := range r.tokens {
		if strings.ToLower(token.Email) == email {
			cp := *token
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (r *stubInvitationTokenRepository) MarkAsUsed(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if token, ok := r.tokens[id]; ok {
		now := r.now()
		token.UsedAt = &now
		return nil
	}
	return sql.ErrNoRows
}

func (r *stubInvitationTokenRepository) InvalidateByEmail(ctx context.Context, email string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	email = strings.ToLower(email)
	now := r.now()
	count := 0
	targetTenantID := tenant.FromContext(ctx)
	for _, token := range r.tokens {
		if targetTenantID > 0 && token.TenantID != targetTenantID {
			continue
		}
		if strings.ToLower(token.Email) == email && token.UsedAt == nil {
			token.UsedAt = &now
			count++
		}
	}
	return count, nil
}

func (r *stubInvitationTokenRepository) DeleteExpired(_ context.Context, now time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for id, token := range r.tokens {
		if token.IsUsed() || !token.ExpiresAt.After(now) {
			delete(r.byToken, token.Token)
			delete(r.tokens, id)
			count++
		}
	}
	return count, nil
}

func (r *stubInvitationTokenRepository) List(_ context.Context, filters map[string]interface{}) ([]*authModel.InvitationToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	var result []*authModel.InvitationToken
	for _, token := range r.tokens {
		include := true
		for key, value := range filters {
			switch key {
			case "pending":
				if pending, ok := value.(bool); ok && pending {
					if token.IsUsed() || !token.ExpiresAt.After(now) {
						include = false
					}
				}
			case "email":
				if v, ok := value.(string); ok && !strings.EqualFold(token.Email, v) {
					include = false
				}
			}
		}
		if include {
			cp := *token
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (r *stubInvitationTokenRepository) UpdateDeliveryResult(_ context.Context, id int64, sentAt *time.Time, emailError *string, retryCount int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	token, exists := r.tokens[id]
	if !exists {
		return sql.ErrNoRows
	}
	token.EmailRetryCount = retryCount
	if sentAt != nil {
		token.EmailSentAt = sentAt
	} else {
		token.EmailSentAt = nil
	}
	if emailError != nil {
		token.EmailError = emailError
	} else {
		token.EmailError = nil
	}
	return nil
}

func (r *stubInvitationTokenRepository) InvalidateByTenantID(_ context.Context, tenantID int64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	now := r.now()
	for _, token := range r.tokens {
		if token.TenantID == tenantID && token.UsedAt == nil {
			token.UsedAt = &now
			count++
		}
	}
	return count, nil
}

// noopRoleRepository provides default panic implementations.
type noopRoleRepository struct{}

func (noopRoleRepository) Create(context.Context, *authModel.Role) error {
	panic("Create not implemented")
}

func (noopRoleRepository) FindByID(context.Context, interface{}) (*authModel.Role, error) {
	panic("FindByID not implemented")
}

func (noopRoleRepository) Update(context.Context, *authModel.Role) error {
	panic("Update not implemented")
}

func (noopRoleRepository) Delete(context.Context, interface{}) error {
	panic("Delete not implemented")
}

func (noopRoleRepository) List(context.Context, map[string]interface{}) ([]*authModel.Role, error) {
	panic("List not implemented")
}

func (noopRoleRepository) FindByName(context.Context, string) (*authModel.Role, error) {
	panic("FindByName not implemented")
}

func (noopRoleRepository) FindByAccountID(context.Context, int64) ([]*authModel.Role, error) {
	panic("FindByAccountID not implemented")
}

func (noopRoleRepository) FindRoleNamesByAccountIDs(context.Context, []int64) (map[int64]string, error) {
	panic("FindRoleNamesByAccountIDs not implemented")
}

func (noopRoleRepository) AssignRoleToAccount(context.Context, int64, int64) error {
	panic("AssignRoleToAccount not implemented")
}

func (noopRoleRepository) RemoveRoleFromAccount(context.Context, int64, int64) error {
	panic("RemoveRoleFromAccount not implemented")
}

func (noopRoleRepository) GetRoleWithPermissions(context.Context, int64) (*authModel.Role, error) {
	panic("GetRoleWithPermissions not implemented")
}

// stubRoleRepository stores roles in memory.
type stubRoleRepository struct {
	noopRoleRepository

	roles map[int64]*authModel.Role
}

func newStubRoleRepository(roles ...*authModel.Role) *stubRoleRepository {
	store := make(map[int64]*authModel.Role, len(roles))
	for _, role := range roles {
		store[role.ID] = role
	}
	return &stubRoleRepository{roles: store}
}

func (r *stubRoleRepository) FindByID(_ context.Context, id interface{}) (*authModel.Role, error) {
	if v, ok := id.(int64); ok {
		if role, exists := r.roles[v]; exists {
			return role, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *stubRoleRepository) FindByName(_ context.Context, name string) (*authModel.Role, error) {
	for _, role := range r.roles {
		if strings.EqualFold(role.Name, name) {
			return role, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *stubRoleRepository) List(_ context.Context, filters map[string]interface{}) ([]*authModel.Role, error) {
	var roles []*authModel.Role
	for _, role := range r.roles {
		if name, ok := filters["name"].(string); ok && !strings.EqualFold(role.Name, name) {
			continue
		}
		if isSystem, ok := filters["is_system"].(bool); ok && role.IsSystem != isSystem {
			continue
		}
		roles = append(roles, role)
	}
	return roles, nil
}

// noopAccountRoleRepository provides default panic implementations.
type noopAccountRoleRepository struct{}

func (noopAccountRoleRepository) Create(context.Context, *authModel.AccountRole) error {
	panic("Create not implemented")
}

func (noopAccountRoleRepository) FindByID(context.Context, interface{}) (*authModel.AccountRole, error) {
	panic("FindByID not implemented")
}

func (noopAccountRoleRepository) Update(context.Context, *authModel.AccountRole) error {
	panic("Update not implemented")
}

func (noopAccountRoleRepository) Delete(context.Context, interface{}) error {
	panic("Delete not implemented")
}

func (noopAccountRoleRepository) List(context.Context, map[string]interface{}) ([]*authModel.AccountRole, error) {
	panic("List not implemented")
}

func (noopAccountRoleRepository) FindByAccountID(context.Context, int64) ([]*authModel.AccountRole, error) {
	panic("FindByAccountID not implemented")
}

func (noopAccountRoleRepository) FindByAccountIDForTenant(context.Context, int64, int64) ([]*authModel.AccountRole, error) {
	panic("FindByAccountIDForTenant not implemented")
}

func (noopAccountRoleRepository) FindByAccountIDForTenantForShare(context.Context, int64, int64) ([]*authModel.AccountRole, error) {
	panic("FindByAccountIDForTenantForShare not implemented")
}

func (noopAccountRoleRepository) FindByRoleID(context.Context, int64) ([]*authModel.AccountRole, error) {
	panic("FindByRoleID not implemented")
}

func (noopAccountRoleRepository) FindByAccountAndRole(context.Context, int64, int64) (*authModel.AccountRole, error) {
	panic("FindByAccountAndRole not implemented")
}

func (noopAccountRoleRepository) DeleteByAccountAndRole(context.Context, int64, int64) error {
	panic("DeleteByAccountAndRole not implemented")
}

func (noopAccountRoleRepository) DeleteByAccountRoleAndTenant(context.Context, int64, int64, int64) error {
	panic("DeleteByAccountRoleAndTenant not implemented")
}

func (noopAccountRoleRepository) DeleteByAccountID(context.Context, int64) error {
	panic("DeleteByAccountID not implemented")
}

func (noopAccountRoleRepository) DeleteByRoleID(context.Context, int64) error {
	panic("DeleteByRoleID not implemented")
}

func (noopAccountRoleRepository) FindAccountRolesWithDetails(context.Context, map[string]interface{}) ([]*authModel.AccountRole, error) {
	panic("FindAccountRolesWithDetails not implemented")
}

// stubAccountRoleRepository records role assignments.
type stubAccountRoleRepository struct {
	noopAccountRoleRepository

	mu              sync.Mutex
	assignments     []*authModel.AccountRole
	findByTenantErr error
}

func newStubAccountRoleRepository() *stubAccountRoleRepository {
	return &stubAccountRoleRepository{}
}

func (r *stubAccountRoleRepository) Create(_ context.Context, ar *authModel.AccountRole) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.assignments = append(r.assignments, ar)
	return nil
}

func (r *stubAccountRoleRepository) FindByAccountAndRole(_ context.Context, accountID, roleID int64) (*authModel.AccountRole, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, assignment := range r.assignments {
		if assignment.AccountID == accountID && assignment.RoleID == roleID {
			return assignment, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *stubAccountRoleRepository) FindByAccountIDForTenant(_ context.Context, accountID, tenantID int64) ([]*authModel.AccountRole, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.findByTenantErr != nil {
		return nil, r.findByTenantErr
	}
	var roles []*authModel.AccountRole
	for _, assignment := range r.assignments {
		if assignment.AccountID == accountID && assignment.TenantID == tenantID {
			roles = append(roles, assignment)
		}
	}
	return roles, nil
}

// FindByAccountIDForTenantForShare mirrors the unlocked variant — the stub has
// no transactions, so the FOR SHARE lock has nothing to model here.
func (r *stubAccountRoleRepository) FindByAccountIDForTenantForShare(ctx context.Context, accountID, tenantID int64) ([]*authModel.AccountRole, error) {
	return r.FindByAccountIDForTenant(ctx, accountID, tenantID)
}

func (r *stubAccountRoleRepository) Assignments() []*authModel.AccountRole {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*authModel.AccountRole, len(r.assignments))
	copy(out, r.assignments)
	return out
}

// noopPersonRepository provides default panic implementations.
type noopPersonRepository struct{}

func (noopPersonRepository) Create(context.Context, *userModel.Person) error {
	panic("Create not implemented")
}

func (noopPersonRepository) FindByID(context.Context, interface{}) (*userModel.Person, error) {
	panic("FindByID not implemented")
}

func (noopPersonRepository) FindByIDForUpdate(context.Context, int64) (*userModel.Person, error) {
	panic("FindByIDForUpdate not implemented")
}

func (noopPersonRepository) FindByTagID(context.Context, string) (*userModel.Person, error) {
	panic("FindByTagID not implemented")
}

func (noopPersonRepository) FindByAccountID(context.Context, int64) (*userModel.Person, error) {
	panic("FindByAccountID not implemented")
}

func (noopPersonRepository) FindByAccountIDs(context.Context, []int64) (map[int64]*userModel.Person, error) {
	panic("FindByAccountIDs not implemented")
}

func (noopPersonRepository) FindByIDs(context.Context, []int64) (map[int64]*userModel.Person, error) {
	panic("FindByIDs not implemented")
}

func (noopPersonRepository) Update(context.Context, *userModel.Person) error {
	panic("Update not implemented")
}

func (noopPersonRepository) Delete(context.Context, interface{}) error {
	panic("Delete not implemented")
}

func (noopPersonRepository) List(context.Context, map[string]interface{}) ([]*userModel.Person, error) {
	panic("List not implemented")
}

func (noopPersonRepository) ListWithOptions(context.Context, *base.QueryOptions) ([]*userModel.Person, error) {
	panic("ListWithOptions not implemented")
}

func (noopPersonRepository) LinkToAccount(context.Context, int64, int64) error {
	panic("LinkToAccount not implemented")
}

func (noopPersonRepository) UnlinkFromAccount(context.Context, int64) error {
	panic("UnlinkFromAccount not implemented")
}

func (noopPersonRepository) LinkToRFIDCard(context.Context, int64, string) error {
	panic("LinkToRFIDCard not implemented")
}

func (noopPersonRepository) UnlinkFromRFIDCard(context.Context, int64) error {
	panic("UnlinkFromRFIDCard not implemented")
}

func (noopPersonRepository) FindWithAccount(context.Context, int64) (*userModel.Person, error) {
	panic("FindWithAccount not implemented")
}

// stubPersonRepository stores people in memory.
type stubPersonRepository struct {
	noopPersonRepository

	mu     sync.Mutex
	people map[int64]*userModel.Person
	nextID int64
	// failCreate simulates a persistence failure during tests.
	failCreate bool
}

func newStubPersonRepository() *stubPersonRepository {
	return &stubPersonRepository{
		people: make(map[int64]*userModel.Person),
	}
}

func (r *stubPersonRepository) Create(_ context.Context, person *userModel.Person) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failCreate {
		return errors.New("person create failed")
	}
	if person.ID == 0 {
		r.nextID++
		person.ID = r.nextID
	}
	r.people[person.ID] = person
	return nil
}

func (r *stubPersonRepository) LinkToAccount(_ context.Context, personID, accountID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if person, ok := r.people[personID]; ok {
		person.AccountID = &accountID
		return nil
	}
	return fmt.Errorf("person %d not found", personID)
}

// LinkToRFIDCard records the transponder on the person, so the identity
// provisioning's reuse path can be asserted on (#2222).
func (r *stubPersonRepository) LinkToRFIDCard(_ context.Context, personID int64, tagID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	person, ok := r.people[personID]
	if !ok {
		return fmt.Errorf("person %d not found", personID)
	}
	person.TagID = &tagID
	return nil
}

// FindByTagID mirrors the real repository: no wearer is a clean (nil, nil).
// The identity provisioning asks before it assigns a transponder, so a bracelet
// somebody else is already wearing is refused instead of reaching the per-school
// unique constraint and coming back as a 500 (#2222).
func (r *stubPersonRepository) FindByTagID(_ context.Context, tagID string) (*userModel.Person, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, person := range r.people {
		if person.TagID != nil && *person.TagID == tagID {
			return person, nil
		}
	}
	return nil, nil
}

// FindByAccountID mirrors the real repository: no match is a clean (nil, nil),
// not an error. The identity provisioning walks this first to reuse an existing
// person instead of creating a second one (#2222).
func (r *stubPersonRepository) FindByAccountID(_ context.Context, accountID int64) (*userModel.Person, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, person := range r.people {
		if person.AccountID != nil && *person.AccountID == accountID {
			return person, nil
		}
	}
	return nil, nil
}

func (r *stubPersonRepository) FindByID(_ context.Context, id interface{}) (*userModel.Person, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v, ok := id.(int64); ok {
		if person, exists := r.people[v]; exists {
			return person, nil
		}
	}
	return nil, sql.ErrNoRows
}

// noopTokenRepository provides default panic implementations.
type noopTokenRepository struct{}

func (noopTokenRepository) Create(context.Context, *authModel.Token) error {
	panic("Create not implemented")
}

func (noopTokenRepository) FindByID(context.Context, interface{}) (*authModel.Token, error) {
	panic("FindByID not implemented")
}

func (noopTokenRepository) Update(context.Context, *authModel.Token) error {
	panic("Update not implemented")
}

func (noopTokenRepository) Delete(context.Context, interface{}) error {
	panic("Delete not implemented")
}

func (noopTokenRepository) List(context.Context, map[string]interface{}) ([]*authModel.Token, error) {
	panic("List not implemented")
}

func (noopTokenRepository) FindByToken(context.Context, string) (*authModel.Token, error) {
	panic("FindByToken not implemented")
}

func (noopTokenRepository) FindByTokenForUpdate(context.Context, string) (*authModel.Token, error) {
	panic("FindByTokenForUpdate not implemented")
}

func (noopTokenRepository) MarkRotated(context.Context, int64, string, []byte, time.Time) error {
	panic("MarkRotated not implemented")
}

func (noopTokenRepository) DeleteExpiredRotatedForAccount(context.Context, int64, time.Time) error {
	panic("DeleteExpiredRotatedForAccount not implemented")
}

func (noopTokenRepository) FindByAccountID(context.Context, int64) ([]*authModel.Token, error) {
	panic("FindByAccountID not implemented")
}

func (noopTokenRepository) FindByAccountIDAndIdentifier(context.Context, int64, string) (*authModel.Token, error) {
	panic("FindByAccountIDAndIdentifier not implemented")
}

func (noopTokenRepository) DeleteExpiredTokens(context.Context) (int, error) {
	panic("DeleteExpiredTokens not implemented")
}

func (noopTokenRepository) ListInactiveAccountIDsWithLiveTokens(context.Context) ([]int64, error) {
	panic("ListInactiveAccountIDsWithLiveTokens not implemented")
}

func (noopTokenRepository) HasLiveTokensCreatedAfter(context.Context, int64, time.Time) (bool, error) {
	panic("HasLiveTokensCreatedAfter not implemented")
}

func (noopTokenRepository) DeleteByAccountIDReturning(context.Context, int64) ([]*authModel.Token, error) {
	panic("DeleteByAccountIDReturning not implemented")
}

func (noopTokenRepository) DeleteAllByAccountIDReturning(context.Context, int64) ([]*authModel.Token, error) {
	panic("DeleteAllByAccountIDReturning not implemented")
}

func (noopTokenRepository) DeleteByAccountIDCreatedAtOrBeforeReturning(context.Context, int64, time.Time) ([]*authModel.Token, error) {
	panic("DeleteByAccountIDCreatedAtOrBeforeReturning not implemented")
}

func (noopTokenRepository) DeleteByAccountIDAndIdentifier(context.Context, int64, string) error {
	panic("DeleteByAccountIDAndIdentifier not implemented")
}

func (noopTokenRepository) FindValidTokens(context.Context, map[string]interface{}) ([]*authModel.Token, error) {
	panic("FindValidTokens not implemented")
}

func (noopTokenRepository) CleanupOldTokensForAccountReturning(context.Context, int64, string, int) ([]*authModel.Token, error) {
	panic("CleanupOldTokensForAccountReturning not implemented")
}

func (noopTokenRepository) DeleteByFamilyIDReturning(context.Context, string) ([]*authModel.Token, error) {
	panic("DeleteByFamilyIDReturning not implemented")
}

func (noopTokenRepository) GetLatestTokenInFamily(context.Context, string) (*authModel.Token, error) {
	panic("GetLatestTokenInFamily not implemented")
}

func (noopTokenRepository) DeleteByTenantIDReturning(context.Context, int64) ([]*authModel.Token, error) {
	panic("DeleteByTenantIDReturning not implemented")
}

// stubTokenRepository tracks delete operations for verification.
type stubTokenRepository struct {
	noopTokenRepository

	mu                sync.Mutex
	deletedAccountIDs []int64
}

func newStubTokenRepository() *stubTokenRepository {
	return &stubTokenRepository{}
}

type stubAccountTenantRepository struct {
	mu       sync.Mutex
	mappings map[int64]*authModel.AccountTenant
	nextID   int64
}

func newStubAccountTenantRepository() *stubAccountTenantRepository {
	return &stubAccountTenantRepository{
		mappings: make(map[int64]*authModel.AccountTenant),
	}
}

func (r *stubAccountTenantRepository) Create(_ context.Context, accountTenant *authModel.AccountTenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if accountTenant.ID == 0 {
		r.nextID++
		accountTenant.ID = r.nextID
	}
	r.mappings[accountTenant.ID] = accountTenant
	return nil
}

func (r *stubAccountTenantRepository) EnsureActive(_ context.Context, accountTenant *authModel.AccountTenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, mapping := range r.mappings {
		if mapping.AccountID == accountTenant.AccountID && mapping.TenantID == accountTenant.TenantID {
			mapping.Status = authModel.AccountTenantStatusActive
			mapping.ActivatedAt = accountTenant.ActivatedAt
			mapping.DeactivatedAt = nil
			return nil
		}
	}
	if accountTenant.ID == 0 {
		r.nextID++
		accountTenant.ID = r.nextID
	}
	accountTenant.Status = authModel.AccountTenantStatusActive
	r.mappings[accountTenant.ID] = accountTenant
	return nil
}

func (r *stubAccountTenantRepository) Deactivate(_ context.Context, accountID, tenantID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for _, mapping := range r.mappings {
		if mapping.AccountID == accountID && mapping.TenantID == tenantID {
			mapping.Status = authModel.AccountTenantStatusInactive
			mapping.DeactivatedAt = &now
		}
	}
	return nil
}

func (r *stubAccountTenantRepository) FindActiveByAccountID(_ context.Context, accountID int64) ([]authModel.AccountTenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []authModel.AccountTenant
	for _, mapping := range r.mappings {
		if mapping.AccountID == accountID && mapping.Status == authModel.AccountTenantStatusActive {
			result = append(result, *mapping)
		}
	}
	return result, nil
}

func (r *stubAccountTenantRepository) FindActiveGuardianByAccountID(context.Context, int64) ([]authModel.AccountTenant, error) {
	panic("FindActiveGuardianByAccountID not implemented")
}

func (r *stubAccountTenantRepository) ExistsByAccountAndTenant(_ context.Context, accountID, tenantID int64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, mapping := range r.mappings {
		if mapping.AccountID == accountID && mapping.TenantID == tenantID && mapping.Status == authModel.AccountTenantStatusActive {
			return true, nil
		}
	}
	return false, nil
}

// ExistsActiveByAccountAndTenantForShare mirrors the unlocked variant — the
// stub has no transactions, so the FOR SHARE lock has nothing to model here.
func (r *stubAccountTenantRepository) ExistsActiveByAccountAndTenantForShare(ctx context.Context, accountID, tenantID int64) (bool, error) {
	return r.ExistsByAccountAndTenant(ctx, accountID, tenantID)
}

func (r *stubAccountTenantRepository) ListAccountsByTenantID(context.Context, int64) ([]authModel.TenantAccountInfo, error) {
	return nil, nil
}
func (r *stubAccountTenantRepository) ListAccountsByOrganizationID(context.Context, int64) ([]authModel.OrgAccountInfo, error) {
	return nil, nil
}
func (r *stubAccountTenantRepository) ListAllAccounts(context.Context) ([]authModel.OrgAccountInfo, error) {
	return nil, nil
}
func (r *stubAccountTenantRepository) ListTenantAccessByAccountID(context.Context, int64) ([]authModel.AccountTenantAccessInfo, error) {
	return nil, nil
}

func (r *stubTokenRepository) DeleteByAccountIDReturning(_ context.Context, accountID int64) ([]*authModel.Token, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deletedAccountIDs = append(r.deletedAccountIDs, accountID)
	return nil, nil
}

func (r *stubTokenRepository) DeleteAllByAccountIDReturning(ctx context.Context, accountID int64) ([]*authModel.Token, error) {
	return r.DeleteByAccountIDReturning(ctx, accountID)
}

func (r *stubTokenRepository) DeleteByAccountIDCreatedAtOrBeforeReturning(ctx context.Context, accountID int64, _ time.Time) ([]*authModel.Token, error) {
	return r.DeleteByAccountIDReturning(ctx, accountID)
}

func (r *stubTokenRepository) DeletedAccountIDs() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]int64, len(r.deletedAccountIDs))
	copy(out, r.deletedAccountIDs)
	return out
}

// newStubStaffRepository returns a testpkg.StaffRepoMock configured to behave
// like the former hand-rolled stubStaffRepository: Create assigns sequential
// IDs and records the entry in-memory. The returned accessor function replaces
// the old *stubStaffRepository.All() method for tests that need to inspect
// everything created so far. Every other method panics on unset Fn, matching
// the original's un-implemented methods.
func newStubStaffRepository() (*testpkg.StaffRepoMock, func() []*userModel.Staff) {
	var mu sync.Mutex
	staff := make(map[int64]*userModel.Staff)
	var nextID int64

	mock := &testpkg.StaffRepoMock{
		CreateFn: func(_ context.Context, s *userModel.Staff) error {
			mu.Lock()
			defer mu.Unlock()
			if s.ID == 0 {
				nextID++
				s.ID = nextID
			}
			staff[s.ID] = s
			return nil
		},
		FindByIDFn: func(context.Context, any) (*userModel.Staff, error) { panic("FindByID not implemented") },
		// Mirrors the real repository, which reports "no staff" as
		// sql.ErrNoRows. The identity provisioning looks a live staff row up
		// before creating one so a re-grant reuses it (#2222).
		FindByPersonIDFn: func(_ context.Context, personID int64) (*userModel.Staff, error) {
			mu.Lock()
			defer mu.Unlock()
			for _, s := range staff {
				if s.PersonID == personID {
					return s, nil
				}
			}
			return nil, sql.ErrNoRows
		},
		UpdateFn: func(context.Context, *userModel.Staff) error { panic("Update not implemented") },
		DeleteFn: func(context.Context, any) error { panic("Delete not implemented") },
		ListFn: func(context.Context, map[string]any) ([]*userModel.Staff, error) {
			panic("List not implemented")
		},
		ClearWorkTimeModelFn: func(context.Context, int64) error { panic("ClearWorkTimeModel not implemented") },
		FindWithPersonFn:     func(context.Context, int64) (*userModel.Staff, error) { panic("FindWithPerson not implemented") },
		FindByIDsFn: func(context.Context, []int64) (map[int64]*userModel.Staff, error) {
			panic("FindByIDs not implemented")
		},
		FindWithPersonByIDsFn: func(context.Context, []int64) (map[int64]*userModel.Staff, error) {
			panic("FindWithPersonByIDs not implemented")
		},
		ListStaffByRolesFn: func(context.Context, []string) ([]*userModel.StaffWithRoleInfo, error) {
			panic("ListStaffByRoles not implemented")
		},
	}

	all := func() []*userModel.Staff {
		mu.Lock()
		defer mu.Unlock()
		out := make([]*userModel.Staff, 0, len(staff))
		for _, s := range staff {
			out = append(out, s)
		}
		return out
	}

	return mock, all
}

// staffRepoOnly discards the accessor from newStubStaffRepository for call
// sites (e.g. struct literal fields) that only need the repository.
func staffRepoOnly() *testpkg.StaffRepoMock {
	mock, _ := newStubStaffRepository()
	return mock
}

// stubStudentRepository answers the one question the identity provisioning
// asks of users.students: is this person a child's record (#2222)? The
// embedded interface leaves everything else unimplemented on purpose — a new
// read would panic here rather than pass silently against a stub that invented
// an answer.
type stubStudentRepository struct {
	userModel.StudentRepository
	mu               sync.Mutex
	studentPersonIDs map[int64]bool
}

func newStubStudentRepository() *stubStudentRepository {
	return &stubStudentRepository{studentPersonIDs: make(map[int64]bool)}
}

// markStudent makes the given person read as a child's record.
func (r *stubStudentRepository) markStudent(personID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.studentPersonIDs[personID] = true
}

// FindByPersonID mirrors the real repository, including how it reports a miss:
// sql.ErrNoRows wrapped in a DatabaseError, never a nil result.
func (r *stubStudentRepository) FindByPersonID(_ context.Context, personID int64) (*userModel.Student, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.studentPersonIDs[personID] {
		student := &userModel.Student{}
		student.PersonID = personID
		return student, nil
	}
	return nil, &base.DatabaseError{Op: "find by person ID", Err: sql.ErrNoRows}
}

// stubTeacherRepository provides a minimal test implementation.
type stubTeacherRepository struct {
	mu       sync.Mutex
	teachers map[int64]*userModel.Teacher
	nextID   int64
}

func newStubTeacherRepository() *stubTeacherRepository {
	return &stubTeacherRepository{
		teachers: make(map[int64]*userModel.Teacher),
	}
}

func (r *stubTeacherRepository) Create(_ context.Context, teacher *userModel.Teacher) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if teacher.ID == 0 {
		r.nextID++
		teacher.ID = r.nextID
	}
	r.teachers[teacher.ID] = teacher
	return nil
}

func (r *stubTeacherRepository) All() []*userModel.Teacher {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*userModel.Teacher, 0, len(r.teachers))
	for _, teacher := range r.teachers {
		out = append(out, teacher)
	}
	return out
}

func (r *stubTeacherRepository) FindByID(context.Context, interface{}) (*userModel.Teacher, error) {
	panic("FindByID not implemented")
}

// FindByStaffID mirrors the real repository: no match is a clean (nil, nil).
// The identity provisioning checks for a live caregiver profile before
// creating one (#2222).
func (r *stubTeacherRepository) FindByStaffID(_ context.Context, staffID int64) (*userModel.Teacher, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, teacher := range r.teachers {
		if teacher.StaffID == staffID {
			return teacher, nil
		}
	}
	return nil, nil
}

func (r *stubTeacherRepository) FindByStaffIDs(context.Context, []int64) (map[int64]*userModel.Teacher, error) {
	panic("FindByStaffIDs not implemented")
}

func (r *stubTeacherRepository) FindBySpecialization(context.Context, string) ([]*userModel.Teacher, error) {
	panic("FindBySpecialization not implemented")
}

func (r *stubTeacherRepository) Update(context.Context, *userModel.Teacher) error {
	panic("Update not implemented")
}

func (r *stubTeacherRepository) Delete(context.Context, interface{}) error {
	panic("Delete not implemented")
}

func (r *stubTeacherRepository) List(context.Context, map[string]interface{}) ([]*userModel.Teacher, error) {
	panic("List not implemented")
}

func (r *stubTeacherRepository) ListWithOptions(context.Context, *base.QueryOptions) ([]*userModel.Teacher, error) {
	panic("ListWithOptions not implemented")
}

func (r *stubTeacherRepository) FindByGroupID(context.Context, int64) ([]*userModel.Teacher, error) {
	panic("FindByGroupID not implemented")
}

func (r *stubTeacherRepository) UpdateQualifications(context.Context, int64, string) error {
	panic("UpdateQualifications not implemented")
}

func (r *stubTeacherRepository) FindWithStaffAndPerson(context.Context, int64) (*userModel.Teacher, error) {
	panic("FindWithStaffAndPerson not implemented")
}

func (r *stubTeacherRepository) ListAllWithStaffAndPerson(context.Context) ([]*userModel.Teacher, error) {
	panic("ListAllWithStaffAndPerson not implemented")
}

func (r *stubTeacherRepository) FindWithStaffAndPersonByIDs(context.Context, []int64) ([]*userModel.Teacher, error) {
	panic("FindWithStaffAndPersonByIDs not implemented")
}

// newStubSchoolRepository returns a testpkg.SchoolRepoMock configured like the
// former hand-rolled stubSchoolRepository: FindByID (and the ForShare/ForUpdate
// variants) return an active school in organization 1, soft-deleted when its ID
// is in deletedTenantIDs. Pass nil for no soft-deleted schools.
func newStubSchoolRepository(deletedTenantIDs map[int64]bool) *testpkg.SchoolRepoMock {
	findByID := func(_ context.Context, id int64) (*platformModel.School, error) {
		school := &platformModel.School{
			Model:          base.Model{ID: id},
			Active:         true,
			OrganizationID: 1,
		}
		if deletedTenantIDs[id] {
			now := time.Now()
			school.DeletedAt = &now
		}
		return school, nil
	}
	return &testpkg.SchoolRepoMock{
		CreateFn:   func(context.Context, *platformModel.School) error { return fmt.Errorf("not implemented") },
		FindByIDFn: findByID,
		FindByIDForShareFn: func(ctx context.Context, id int64) (*platformModel.School, error) {
			return findByID(ctx, id)
		},
		FindByIDForUpdateFn: func(ctx context.Context, id int64) (*platformModel.School, error) {
			return findByID(ctx, id)
		},
		FindBySlugFn: func(context.Context, string) (*platformModel.School, error) {
			return nil, fmt.Errorf("not found")
		},
		FindByOrganizationAndSlugFn: func(context.Context, int64, string) (*platformModel.School, error) {
			return nil, fmt.Errorf("not found")
		},
		FindBySubdomainFn: func(context.Context, string) (*platformModel.School, error) {
			return nil, fmt.Errorf("not found")
		},
		UpdateFn: func(context.Context, *platformModel.School) error { return fmt.Errorf("not implemented") },
	}
}

// helper to build default email used in tests.
func newDefaultFromEmail() email.Email {
	return email.NewEmail("moto", "no-reply@moto.example")
}

// Stubs for the issue #585 refactor interface additions — unused by auth tests.
func (r *stubAccountRepository) AnonymizeForDeletion(context.Context, int64, string) error {
	return nil
}

func (r *stubPersonRepository) AnonymizeAndSoftDelete(context.Context, int64) error {
	return nil
}

func (noopAccountRepository) AnonymizeForDeletion(context.Context, int64, string) error {
	return nil
}

func (r *stubTeacherRepository) ListActiveCaregivers(context.Context) ([]*userModel.ActiveCaregiver, error) {
	return nil, nil
}

func (r *stubTeacherRepository) FindActiveCaregiverByAccountID(context.Context, int64) (*userModel.ActiveCaregiver, error) {
	return nil, nil
}

// stubRFIDCardRepository holds the transponders that exist at the school, so the
// identity provisioning can refuse one that does not (#2222).
type stubRFIDCardRepository struct {
	mu    sync.Mutex
	cards map[string]bool
}

func newStubRFIDCardRepository(ids ...string) *stubRFIDCardRepository {
	cards := make(map[string]bool, len(ids))
	for _, id := range ids {
		cards[id] = true
	}
	return &stubRFIDCardRepository{cards: cards}
}

// FindByID mirrors the real repository, which reports an unknown card as a clean
// (nil, nil) rather than an error.
func (r *stubRFIDCardRepository) FindByID(_ context.Context, id string) (*userModel.RFIDCard, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.cards[id] {
		return nil, nil
	}
	return &userModel.RFIDCard{StringIDModel: base.StringIDModel{ID: id}}, nil
}

func (r *stubRFIDCardRepository) Create(context.Context, *userModel.RFIDCard) error { return nil }
func (r *stubRFIDCardRepository) Update(context.Context, *userModel.RFIDCard) error { return nil }
func (r *stubRFIDCardRepository) Delete(context.Context, string) error              { return nil }
func (r *stubRFIDCardRepository) Deactivate(context.Context, string) error          { return nil }
func (r *stubRFIDCardRepository) List(context.Context, map[string]interface{}) ([]*userModel.RFIDCard, error) {
	return nil, nil
}
