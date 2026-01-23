package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/email"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
)

// testRateLimitRepo provides an in-memory implementation of the password reset rate limiter.
type testRateLimitRepo struct {
	mu           sync.Mutex
	attempts     int
	windowStart  time.Time
	checkErr     error
	incrementErr error
}

func newTestRateLimitRepo() *testRateLimitRepo {
	return &testRateLimitRepo{}
}

func (r *testRateLimitRepo) CheckRateLimit(_ context.Context, _ string) (*authModel.RateLimitState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.checkErr != nil {
		return nil, r.checkErr
	}

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

	if r.incrementErr != nil {
		return nil, r.incrementErr
	}

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

func (r *testRateLimitRepo) setCheckError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checkErr = err
}

func (r *testRateLimitRepo) setIncrementError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.incrementErr = err
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

// capturingMailer records messages sent during tests.
type capturingMailer struct {
	mu       sync.Mutex
	messages []email.Message
	ch       chan struct{}
}

func newCapturingMailer() *capturingMailer {
	return &capturingMailer{
		ch: make(chan struct{}, 16),
	}
}

func (m *capturingMailer) Send(msg email.Message) error {
	m.mu.Lock()
	m.messages = append(m.messages, msg)
	m.mu.Unlock()

	select {
	case m.ch <- struct{}{}:
	default:
	}
	return nil
}

func (m *capturingMailer) Messages() []email.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]email.Message, len(m.messages))
	copy(out, m.messages)
	return out
}

func (m *capturingMailer) WaitForMessages(count int, timeout time.Duration) bool {
	if count <= 0 {
		return true
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		if len(m.Messages()) >= count {
			return true
		}
		select {
		case <-m.ch:
			if len(m.Messages()) >= count {
				return true
			}
		case <-timer.C:
			return len(m.Messages()) >= count
		}
	}
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

func (noopAccountRepository) FindByEmail(context.Context, string) (*authModel.Account, error) {
	panic("FindByEmail not implemented")
}

func (noopAccountRepository) FindByUsername(context.Context, string) (*authModel.Account, error) {
	panic("FindByUsername not implemented")
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

func (noopAccountRepository) FindByRole(context.Context, string) ([]*authModel.Account, error) {
	panic("FindByRole not implemented")
}

func (noopAccountRepository) FindAccountsWithRolesAndPermissions(context.Context, map[string]interface{}) ([]*authModel.Account, error) {
	panic("FindAccountsWithRolesAndPermissions not implemented")
}

// stubAccountRepository implements a minimal in-memory account store.
type stubAccountRepository struct {
	noopAccountRepository

	mu       sync.Mutex
	accounts map[string]*authModel.Account
	byID     map[int64]*authModel.Account
	nextID   int64

	failCreate        bool
	updatePasswordErr error
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

func (r *stubAccountRepository) FindByEmail(_ context.Context, email string) (*authModel.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if acc, ok := r.accounts[strings.ToLower(email)]; ok {
		return acc, nil
	}
	return nil, sql.ErrNoRows
}

func (r *stubAccountRepository) FindByID(_ context.Context, id interface{}) (*authModel.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v, ok := id.(int64); ok {
		if acc, exists := r.byID[v]; exists {
			return acc, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *stubAccountRepository) UpdatePassword(_ context.Context, id int64, hash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updatePasswordErr != nil {
		return r.updatePasswordErr
	}
	if acc, ok := r.byID[id]; ok {
		acc.PasswordHash = &hash
		return nil
	}
	return sql.ErrNoRows
}

func (r *stubAccountRepository) setUpdatePasswordError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updatePasswordErr = err
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

func (noopPasswordResetTokenRepository) FindTokensWithAccount(context.Context, map[string]interface{}) ([]*authModel.PasswordResetToken, error) {
	panic("FindTokensWithAccount not implemented")
}

// stubPasswordResetTokenRepository stores tokens in memory.
type stubPasswordResetTokenRepository struct {
	noopPasswordResetTokenRepository

	mu                sync.Mutex
	tokens            map[string]*authModel.PasswordResetToken
	byID              map[int64]*authModel.PasswordResetToken
	nextID            int64
	createErr         error
	invalidateErr     error
	markAsUsedErr     error
	updateDeliveryErr error
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
	if r.createErr != nil {
		return r.createErr
	}
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
			return token, nil
		}
	case int:
		if token, ok := r.byID[int64(v)]; ok {
			return token, nil
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
	return item, nil
}

func (r *stubPasswordResetTokenRepository) MarkAsUsed(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.markAsUsedErr != nil {
		return r.markAsUsedErr
	}
	if token, ok := r.byID[id]; ok {
		token.Used = true
		return nil
	}
	return sql.ErrNoRows
}

func (r *stubPasswordResetTokenRepository) UpdateDeliveryResult(_ context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateDeliveryErr != nil {
		return r.updateDeliveryErr
	}
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
	if r.invalidateErr != nil {
		return r.invalidateErr
	}
	for key, token := range r.tokens {
		if token.AccountID == accountID {
			token.Used = true
			delete(r.tokens, key)
		}
	}
	return nil
}

func (r *stubPasswordResetTokenRepository) setCreateError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.createErr = err
}

func (r *stubPasswordResetTokenRepository) setInvalidateError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.invalidateErr = err
}

func (r *stubPasswordResetTokenRepository) setMarkAsUsedError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.markAsUsedErr = err
}

func (r *stubPasswordResetTokenRepository) setUpdateDeliveryError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updateDeliveryErr = err
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

func (noopTokenRepository) FindByAccountID(context.Context, int64) ([]*authModel.Token, error) {
	panic("FindByAccountID not implemented")
}

func (noopTokenRepository) FindByAccountIDAndIdentifier(context.Context, int64, string) (*authModel.Token, error) {
	panic("FindByAccountIDAndIdentifier not implemented")
}

func (noopTokenRepository) DeleteExpiredTokens(context.Context) (int, error) {
	panic("DeleteExpiredTokens not implemented")
}

func (noopTokenRepository) DeleteByAccountID(context.Context, int64) error {
	panic("DeleteByAccountID not implemented")
}

func (noopTokenRepository) DeleteByAccountIDAndIdentifier(context.Context, int64, string) error {
	panic("DeleteByAccountIDAndIdentifier not implemented")
}

func (noopTokenRepository) FindValidTokens(context.Context, map[string]interface{}) ([]*authModel.Token, error) {
	panic("FindValidTokens not implemented")
}

func (noopTokenRepository) FindTokensWithAccount(context.Context, map[string]interface{}) ([]*authModel.Token, error) {
	panic("FindTokensWithAccount not implemented")
}

func (noopTokenRepository) CleanupOldTokensForAccount(context.Context, int64, int) error {
	panic("CleanupOldTokensForAccount not implemented")
}

func (noopTokenRepository) FindByFamilyID(context.Context, string) ([]*authModel.Token, error) {
	panic("FindByFamilyID not implemented")
}

func (noopTokenRepository) DeleteByFamilyID(context.Context, string) error {
	panic("DeleteByFamilyID not implemented")
}

func (noopTokenRepository) GetLatestTokenInFamily(context.Context, string) (*authModel.Token, error) {
	panic("GetLatestTokenInFamily not implemented")
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

func (r *stubTokenRepository) DeleteByAccountID(_ context.Context, accountID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deletedAccountIDs = append(r.deletedAccountIDs, accountID)
	return nil
}

func (r *stubTokenRepository) DeletedAccountIDs() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]int64, len(r.deletedAccountIDs))
	copy(out, r.deletedAccountIDs)
	return out
}

// helper to build default email used in tests.
func newDefaultFromEmail() email.Email {
	return email.NewEmail("moto", "no-reply@moto.example")
}
