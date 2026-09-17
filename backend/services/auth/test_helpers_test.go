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

// flakyMailer fails a configurable number of initial attempts before succeeding.
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

func (r *stubAccountRepository) ClearPIN(_ context.Context, _ int64) error {
	panic("ClearPIN not implemented in stubAccountRepository")
}

// stubInvitationTokenRepository stores invitations in memory.
// noopRoleRepository provides default panic implementations.
type noopRoleRepository struct{}

func (noopRoleRepository) Create(context.Context, *authModel.Role) error {
	panic("Create not implemented")
}

func (noopRoleRepository) FindByID(context.Context, interface{}) (*authModel.Role, error) {
	panic("FindByID not implemented")
}

func (noopRoleRepository) FindByIDForUpdate(context.Context, int64) (*authModel.Role, error) {
	panic("FindByIDForUpdate not implemented")
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

func (r *stubRoleRepository) FindByIDForUpdate(ctx context.Context, id int64) (*authModel.Role, error) {
	return r.FindByID(ctx, id)
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

func newDefaultFromEmail() email.Email {
	return email.NewEmail("moto", "no-reply@moto.example")
}

// Stubs for the issue #585 refactor interface additions — unused by auth tests.
func (r *stubAccountRepository) AnonymizeForDeletion(context.Context, int64, string) error {
	return nil
}

func (noopAccountRepository) AnonymizeForDeletion(context.Context, int64, string) error {
	return nil
}
