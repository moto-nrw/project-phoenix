package platform_test

import (
	"context"
	"time"

	auth "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/platform"
)

// Shared mock for operator repository
type mockOperatorRepo struct {
	findByIDFn          func(ctx context.Context, id int64) (*platform.Operator, error)
	findByIDForUpdateFn func(ctx context.Context, id int64) (*platform.Operator, error)
	findByEmailFn       func(ctx context.Context, email string) (*platform.Operator, error)
	updateFn            func(ctx context.Context, operator *platform.Operator) error
	updateLastLoginFn   func(ctx context.Context, id int64) error
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

func (m *mockOperatorRepo) UpdateLastLogin(ctx context.Context, id int64) error {
	if m.updateLastLoginFn != nil {
		return m.updateLastLoginFn(ctx, id)
	}
	return nil
}

// IncrementMFAAttempts / ResetMFAAttempts (added for #1430 review item #6
// atomic MFA lockout counter) — default to a no-op zero-value result so
// tests that don't exercise the MFA failure path continue to compile and
// run. Tests that DO exercise it (e.g. handleFailedAttempt under race)
// should swap in a fake that records calls.
func (m *mockOperatorRepo) IncrementMFAAttempts(ctx context.Context, id int64, threshold int, lockoutDuration time.Duration) (platform.OperatorMFAAttemptResult, error) {
	return platform.OperatorMFAAttemptResult{}, nil
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

type mockOperatorRefreshTokenRepo struct {
	createFn                 func(ctx context.Context, token *platform.OperatorRefreshToken) error
	findByTokenForUpdateFn   func(ctx context.Context, token string) (*platform.OperatorRefreshToken, error)
	markRotatedFn            func(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error
	deleteExpiredRotatedFn   func(ctx context.Context, familyID string, now time.Time) error
	deleteFn                 func(ctx context.Context, id any) error
	deleteByOperatorIDFn     func(ctx context.Context, operatorID int64) (int, error)
	deleteByFamilyIDFn       func(ctx context.Context, familyID string) error
	getLatestTokenInFamilyFn func(ctx context.Context, familyID string) (*platform.OperatorRefreshToken, error)
	deleteExpiredFn          func(ctx context.Context, now time.Time) (int, error)
	created                  []*platform.OperatorRefreshToken
}

func (m *mockOperatorRefreshTokenRepo) Create(ctx context.Context, token *platform.OperatorRefreshToken) error {
	if m.createFn != nil {
		return m.createFn(ctx, token)
	}
	m.created = append(m.created, token)
	return nil
}

func (m *mockOperatorRefreshTokenRepo) FindByTokenForUpdate(ctx context.Context, token string) (*platform.OperatorRefreshToken, error) {
	if m.findByTokenForUpdateFn != nil {
		return m.findByTokenForUpdateFn(ctx, token)
	}
	return nil, nil
}

func (m *mockOperatorRefreshTokenRepo) MarkRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error {
	if m.markRotatedFn != nil {
		return m.markRotatedFn(ctx, id, replacementToken, recoveryProofHash, rotatedAt)
	}
	return nil
}

func (m *mockOperatorRefreshTokenRepo) DeleteExpiredRotated(ctx context.Context, familyID string, now time.Time) error {
	if m.deleteExpiredRotatedFn != nil {
		return m.deleteExpiredRotatedFn(ctx, familyID, now)
	}
	return nil
}

func (m *mockOperatorRefreshTokenRepo) Delete(ctx context.Context, id any) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockOperatorRefreshTokenRepo) DeleteByOperatorID(ctx context.Context, operatorID int64) (int, error) {
	if m.deleteByOperatorIDFn != nil {
		return m.deleteByOperatorIDFn(ctx, operatorID)
	}
	return 0, nil
}

func (m *mockOperatorRefreshTokenRepo) DeleteByFamilyID(ctx context.Context, familyID string) error {
	if m.deleteByFamilyIDFn != nil {
		return m.deleteByFamilyIDFn(ctx, familyID)
	}
	return nil
}

func (m *mockOperatorRefreshTokenRepo) GetLatestTokenInFamily(ctx context.Context, familyID string) (*platform.OperatorRefreshToken, error) {
	if m.getLatestTokenInFamilyFn != nil {
		return m.getLatestTokenInFamilyFn(ctx, familyID)
	}
	return nil, nil
}

func (m *mockOperatorRefreshTokenRepo) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	if m.deleteExpiredFn != nil {
		return m.deleteExpiredFn(ctx, now)
	}
	return 0, nil
}

// Shared mock for account tenant repository
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

func (m *mockAccountTenantRepo) Deactivate(context.Context, int64, int64) error {
	return nil
}

func (m *mockAccountTenantRepo) FindActiveByAccountID(ctx context.Context, accountID int64) ([]auth.AccountTenant, error) {
	if m.findActiveByAccountIDFn != nil {
		return m.findActiveByAccountIDFn(ctx, accountID)
	}
	return nil, nil
}

func (m *mockAccountTenantRepo) ExistsByAccountAndTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	if m.existsByAccountAndTenantFn != nil {
		return m.existsByAccountAndTenantFn(ctx, accountID, tenantID)
	}
	return false, nil
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

// Shared mock for organization repository (used by announcement targeting validation)
type mockOrgRepoShared struct {
	findByIDFn   func(ctx context.Context, id int64) (*platform.Organization, error)
	countByIDsFn func(ctx context.Context, ids []int64) (int, error)
}

func (m *mockOrgRepoShared) Create(ctx context.Context, organization *platform.Organization) error {
	return nil
}

func (m *mockOrgRepoShared) FindByID(ctx context.Context, id int64) (*platform.Organization, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return &platform.Organization{}, nil
}

func (m *mockOrgRepoShared) FindByIDForShare(ctx context.Context, id int64) (*platform.Organization, error) {
	return m.FindByID(ctx, id)
}

func (m *mockOrgRepoShared) FindByIDForUpdate(ctx context.Context, id int64) (*platform.Organization, error) {
	return m.FindByID(ctx, id)
}

func (m *mockOrgRepoShared) FindBySlug(ctx context.Context, slug string) (*platform.Organization, error) {
	return nil, nil
}

func (m *mockOrgRepoShared) List(ctx context.Context) ([]*platform.Organization, error) {
	return nil, nil
}

func (m *mockOrgRepoShared) Update(ctx context.Context, organization *platform.Organization) error {
	return nil
}

func (m *mockOrgRepoShared) CountByIDs(ctx context.Context, ids []int64) (int, error) {
	if m.countByIDsFn != nil {
		return m.countByIDsFn(ctx, ids)
	}
	return len(ids), nil
}

func (m *mockOrgRepoShared) SoftDelete(context.Context, int64) error { return nil }
func (m *mockOrgRepoShared) Restore(context.Context, int64) error    { return nil }

// Shared mock for announcement repository
type mockAnnouncementRepoShared struct {
	createFn    func(ctx context.Context, announcement *platform.Announcement) error
	findByIDFn  func(ctx context.Context, id int64) (*platform.Announcement, error)
	updateFn    func(ctx context.Context, announcement *platform.Announcement) error
	deleteFn    func(ctx context.Context, id int64) error
	listFn      func(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error)
	publishFn   func(ctx context.Context, id int64) error
	unpublishFn func(ctx context.Context, id int64) error
}

func (m *mockAnnouncementRepoShared) Create(ctx context.Context, announcement *platform.Announcement) error {
	if m.createFn != nil {
		return m.createFn(ctx, announcement)
	}
	return nil
}

func (m *mockAnnouncementRepoShared) FindByID(ctx context.Context, id int64) (*platform.Announcement, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockAnnouncementRepoShared) Update(ctx context.Context, announcement *platform.Announcement) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, announcement)
	}
	return nil
}

func (m *mockAnnouncementRepoShared) Delete(ctx context.Context, id int64) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockAnnouncementRepoShared) List(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error) {
	if m.listFn != nil {
		return m.listFn(ctx, includeInactive)
	}
	return []*platform.Announcement{}, nil
}

func (m *mockAnnouncementRepoShared) ListPublished(ctx context.Context) ([]*platform.Announcement, error) {
	return []*platform.Announcement{}, nil
}

func (m *mockAnnouncementRepoShared) Publish(ctx context.Context, id int64) error {
	if m.publishFn != nil {
		return m.publishFn(ctx, id)
	}
	return nil
}

func (m *mockAnnouncementRepoShared) Unpublish(ctx context.Context, id int64) error {
	if m.unpublishFn != nil {
		return m.unpublishFn(ctx, id)
	}
	return nil
}

// Shared mock for announcement view repository
type mockAnnouncementViewRepoShared struct {
	getUnreadForUserFn func(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) ([]*platform.Announcement, error)
	countUnreadFn      func(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) (int, error)
	markSeenFn         func(ctx context.Context, userID, announcementID int64) error
	markDismissedFn    func(ctx context.Context, userID, announcementID int64) error
	getStatsFn         func(ctx context.Context, announcementID int64) (*platform.AnnouncementStats, error)
	getViewDetailsFn   func(ctx context.Context, announcementID int64) ([]*platform.AnnouncementViewDetail, error)
}

func (m *mockAnnouncementViewRepoShared) MarkSeen(ctx context.Context, userID, announcementID int64) error {
	if m.markSeenFn != nil {
		return m.markSeenFn(ctx, userID, announcementID)
	}
	return nil
}

func (m *mockAnnouncementViewRepoShared) MarkDismissed(ctx context.Context, userID, announcementID int64) error {
	if m.markDismissedFn != nil {
		return m.markDismissedFn(ctx, userID, announcementID)
	}
	return nil
}

func (m *mockAnnouncementViewRepoShared) GetUnreadForUser(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) ([]*platform.Announcement, error) {
	if m.getUnreadForUserFn != nil {
		return m.getUnreadForUserFn(ctx, userID, userRoles, tenantID, orgID)
	}
	return []*platform.Announcement{}, nil
}

func (m *mockAnnouncementViewRepoShared) CountUnread(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) (int, error) {
	if m.countUnreadFn != nil {
		return m.countUnreadFn(ctx, userID, userRoles, tenantID, orgID)
	}
	return 0, nil
}

func (m *mockAnnouncementViewRepoShared) HasSeen(ctx context.Context, userID, announcementID int64) (bool, error) {
	return false, nil
}

func (m *mockAnnouncementViewRepoShared) GetStats(ctx context.Context, announcementID int64) (*platform.AnnouncementStats, error) {
	if m.getStatsFn != nil {
		return m.getStatsFn(ctx, announcementID)
	}
	return &platform.AnnouncementStats{}, nil
}

func (m *mockAnnouncementViewRepoShared) GetViewDetails(ctx context.Context, announcementID int64) ([]*platform.AnnouncementViewDetail, error) {
	if m.getViewDetailsFn != nil {
		return m.getViewDetailsFn(ctx, announcementID)
	}
	return []*platform.AnnouncementViewDetail{}, nil
}
