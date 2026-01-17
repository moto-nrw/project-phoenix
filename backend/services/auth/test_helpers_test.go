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
	userModel "github.com/moto-nrw/project-phoenix/models/users"
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
	if acc, ok := r.byID[id]; ok {
		acc.PasswordHash = &hash
		return nil
	}
	return sql.ErrNoRows
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
			return token, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *stubInvitationTokenRepository) FindByToken(_ context.Context, value string) (*authModel.InvitationToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if token, ok := r.byToken[value]; ok {
		return token, nil
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
	return token, nil
}

func (r *stubInvitationTokenRepository) FindByEmail(_ context.Context, email string) ([]*authModel.InvitationToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	email = strings.ToLower(email)
	var result []*authModel.InvitationToken
	for _, token := range r.tokens {
		if strings.ToLower(token.Email) == email {
			result = append(result, token)
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

func (r *stubInvitationTokenRepository) InvalidateByEmail(_ context.Context, email string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	email = strings.ToLower(email)
	now := r.now()
	count := 0
	for _, token := range r.tokens {
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
			result = append(result, token)
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

func (noopAccountRoleRepository) FindByRoleID(context.Context, int64) ([]*authModel.AccountRole, error) {
	panic("FindByRoleID not implemented")
}

func (noopAccountRoleRepository) FindByAccountAndRole(context.Context, int64, int64) (*authModel.AccountRole, error) {
	panic("FindByAccountAndRole not implemented")
}

func (noopAccountRoleRepository) DeleteByAccountAndRole(context.Context, int64, int64) error {
	panic("DeleteByAccountAndRole not implemented")
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

	mu          sync.Mutex
	assignments []*authModel.AccountRole
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

func (noopPersonRepository) FindByTagID(context.Context, string) (*userModel.Person, error) {
	panic("FindByTagID not implemented")
}

func (noopPersonRepository) FindByAccountID(context.Context, int64) (*userModel.Person, error) {
	panic("FindByAccountID not implemented")
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

// stubStaffRepository provides a minimal test implementation.
type stubStaffRepository struct {
	mu     sync.Mutex
	staff  map[int64]*userModel.Staff
	nextID int64
}

func newStubStaffRepository() *stubStaffRepository {
	return &stubStaffRepository{
		staff: make(map[int64]*userModel.Staff),
	}
}

func (r *stubStaffRepository) Create(_ context.Context, staff *userModel.Staff) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if staff.ID == 0 {
		r.nextID++
		staff.ID = r.nextID
	}
	r.staff[staff.ID] = staff
	return nil
}

func (r *stubStaffRepository) FindByID(context.Context, interface{}) (*userModel.Staff, error) {
	panic("FindByID not implemented")
}

func (r *stubStaffRepository) FindByPersonID(context.Context, int64) (*userModel.Staff, error) {
	panic("FindByPersonID not implemented")
}

func (r *stubStaffRepository) Update(context.Context, *userModel.Staff) error {
	panic("Update not implemented")
}

func (r *stubStaffRepository) Delete(context.Context, interface{}) error {
	panic("Delete not implemented")
}

func (r *stubStaffRepository) List(context.Context, map[string]interface{}) ([]*userModel.Staff, error) {
	panic("List not implemented")
}

func (r *stubStaffRepository) UpdateNotes(context.Context, int64, string) error {
	panic("UpdateNotes not implemented")
}

func (r *stubStaffRepository) FindWithPerson(context.Context, int64) (*userModel.Staff, error) {
	panic("FindWithPerson not implemented")
}

func (r *stubStaffRepository) ListAllWithPerson(context.Context) ([]*userModel.Staff, error) {
	panic("ListAllWithPerson not implemented")
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

func (r *stubTeacherRepository) FindByID(context.Context, interface{}) (*userModel.Teacher, error) {
	panic("FindByID not implemented")
}

func (r *stubTeacherRepository) FindByStaffID(context.Context, int64) (*userModel.Teacher, error) {
	panic("FindByStaffID not implemented")
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

// helper to build default email used in tests.
func newDefaultFromEmail() email.Email {
	return email.NewEmail("moto", "no-reply@moto.example")
}
