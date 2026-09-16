package platform_test

import (
	"context"
	"time"

	auth "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// Shared mock for operator repository
type mockOperatorRepo struct {
	findByIDFn          func(ctx context.Context, id int64) (*platform.Operator, error)
	findByIDForUpdateFn func(ctx context.Context, id int64) (*platform.Operator, error)
	findByEmailFn       func(ctx context.Context, email string) (*platform.Operator, error)
	updateFn            func(ctx context.Context, operator *platform.Operator) error
	listFn              func(ctx context.Context) ([]*platform.Operator, error)
}

func (m *mockOperatorRepo) Create(ctx context.Context, operator *platform.Operator) error {
	return nil
}

func (m *mockOperatorRepo) FindByID(ctx context.Context, id int64) (*platform.Operator, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

// FindByIDForUpdate delegates to findByIDForUpdateFn when set, allowing tests
// to independently control locking behavior (e.g. simulating lock failures).
// Falls back to findByIDFn so tests that don't care about locking semantics
// can configure a single lookup function for both FindByID and FindByIDForUpdate.
func (m *mockOperatorRepo) FindByIDForUpdate(ctx context.Context, id int64) (*platform.Operator, error) {
	if m.findByIDForUpdateFn != nil {
		return m.findByIDForUpdateFn(ctx, id)
	}
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockOperatorRepo) FindByEmail(ctx context.Context, email string) (*platform.Operator, error) {
	if m.findByEmailFn != nil {
		return m.findByEmailFn(ctx, email)
	}
	return nil, nil
}

func (m *mockOperatorRepo) Update(ctx context.Context, operator *platform.Operator) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, operator)
	}
	return nil
}

func (m *mockOperatorRepo) Delete(ctx context.Context, id int64) error {
	return nil
}

func (m *mockOperatorRepo) List(ctx context.Context) ([]*platform.Operator, error) {
	if m.listFn != nil {
		return m.listFn(ctx)
	}
	return []*platform.Operator{}, nil
}

// IncrementMFAAttempts / ResetMFAAttempts (added for #1430 review item #6
// atomic MFA lockout counter) — default to a no-op zero-value result so
// tests that don't exercise the MFA failure path continue to compile and
// run. Tests that DO exercise it (e.g. handleFailedAttempt under race)
// should swap in a fake that records calls.
func (m *mockOperatorRepo) IncrementMFAAttempts(ctx context.Context, id int64, threshold int, lockoutDuration time.Duration) (platformSvc.OperatorMFAAttempts, error) {
	return platformSvc.OperatorMFAAttempts{}, nil
}

func (m *mockOperatorRepo) ResetMFAAttempts(ctx context.Context, id int64) error {
	return nil
}

// Shared mock for audit log repository
type mockAuditLogRepoShared struct {
	createFn func(ctx context.Context, entry *platform.OperatorAuditLog) error
}

func (m *mockAuditLogRepoShared) Create(ctx context.Context, entry *platform.OperatorAuditLog) error {
	if m.createFn != nil {
		return m.createFn(ctx, entry)
	}
	return nil
}

func (m *mockAuditLogRepoShared) FindByOperatorID(ctx context.Context, operatorID int64, limit int) ([]*platform.OperatorAuditLog, error) {
	return nil, nil
}

func (m *mockAuditLogRepoShared) FindByResourceType(ctx context.Context, resourceType string, limit int) ([]*platform.OperatorAuditLog, error) {
	return nil, nil
}

func (m *mockAuditLogRepoShared) FindByDateRange(ctx context.Context, start, end time.Time, limit int) ([]*platform.OperatorAuditLog, error) {
	return nil, nil
}

type mockAccountTenantRepo struct {
	createFn                     func(ctx context.Context, mapping *auth.AccountTenant) error
	ensureActiveFn               func(ctx context.Context, mapping *auth.AccountTenant) error
	findActiveByAccountIDFn      func(ctx context.Context, accountID int64) ([]auth.AccountTenant, error)
	existsByAccountAndTenantFn   func(ctx context.Context, accountID, tenantID int64) (bool, error)
	listAccountsByTenantIDFn     func(ctx context.Context, tenantID int64) ([]auth.TenantAccountInfo, error)
	listAccountsByOrganizationFn func(ctx context.Context, organizationID int64) ([]auth.OrgAccountInfo, error)
	listAllAccountsFn            func(ctx context.Context) ([]auth.OrgAccountInfo, error)
}

func (m *mockAccountTenantRepo) Create(ctx context.Context, mapping *auth.AccountTenant) error {
	if m.createFn != nil {
		return m.createFn(ctx, mapping)
	}
	return nil
}

func (m *mockAccountTenantRepo) EnsureActive(ctx context.Context, mapping *auth.AccountTenant) error {
	if m.ensureActiveFn != nil {
		return m.ensureActiveFn(ctx, mapping)
	}
	return nil
}

func (m *mockAccountTenantRepo) FindActiveByAccountID(ctx context.Context, accountID int64) ([]auth.AccountTenant, error) {
	if m.findActiveByAccountIDFn != nil {
		return m.findActiveByAccountIDFn(ctx, accountID)
	}
	return nil, nil
}

func (m *mockAccountTenantRepo) FindActiveGuardianByAccountID(context.Context, int64) ([]auth.AccountTenant, error) {
	panic("unexpected FindActiveGuardianByAccountID")
}

func (m *mockAccountTenantRepo) ExistsByAccountAndTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	if m.existsByAccountAndTenantFn != nil {
		return m.existsByAccountAndTenantFn(ctx, accountID, tenantID)
	}
	return false, nil
}
func (m *mockAccountTenantRepo) ExistsActiveByAccountAndTenantForShare(ctx context.Context, accountID, tenantID int64) (bool, error) {
	return m.ExistsByAccountAndTenant(ctx, accountID, tenantID)
}

func (m *mockAccountTenantRepo) ListAccountsByTenantID(ctx context.Context, tenantID int64) ([]auth.TenantAccountInfo, error) {
	if m.listAccountsByTenantIDFn != nil {
		return m.listAccountsByTenantIDFn(ctx, tenantID)
	}
	return nil, nil
}

func (m *mockAccountTenantRepo) ListAccountsByOrganizationID(ctx context.Context, organizationID int64) ([]auth.OrgAccountInfo, error) {
	if m.listAccountsByOrganizationFn != nil {
		return m.listAccountsByOrganizationFn(ctx, organizationID)
	}
	return nil, nil
}

func (m *mockAccountTenantRepo) ListAllAccounts(ctx context.Context) ([]auth.OrgAccountInfo, error) {
	if m.listAllAccountsFn != nil {
		return m.listAllAccountsFn(ctx)
	}
	return nil, nil
}
