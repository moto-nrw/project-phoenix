package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

// The operator provisioning flows (#3253) reach the owners they touch
// through these consumer-owned seams; the root binds them. Every seam that
// takes a tenant ID scopes its own work to that school.

// OperatorDashboard is the named tenant-safe read projection behind the
// operator overview: organisation and school rows with their account counts
// and the PWA standalone-usage buckets. It runs in the caller's
// administrative transaction.
type OperatorDashboard interface {
	Counts(ctx context.Context) (domain.DashboardCounts, error)
	OrganizationSummaries(ctx context.Context) ([]domain.OrganizationSummary, error)
	// SchoolSummaries lists every school, or the schools of one organisation
	// when organizationID is set, including soft-deleted ones.
	SchoolSummaries(ctx context.Context, organizationID *int64) ([]domain.SchoolSummary, error)
	// PWAUsage returns the usage buckets within window; tenantID > 0 limits
	// them to one school.
	PWAUsage(ctx context.Context, tenantID int64, window time.Duration) ([]domain.PWAUsageRow, error)
}

// ProvisioningIdentity is the Identity & Access part of provisioning: roles,
// school-admin invitations, account registration, the school identity
// chain, account listings and session revocation. Owner errors pass through
// unchanged, except the identity chain's input errors, which wrap
// domain.ErrInvalidSchoolIdentity.
type ProvisioningIdentity interface {
	ListSystemRoles(ctx context.Context) ([]domain.Role, error)
	FindSystemRole(ctx context.Context, name string) (domain.Role, bool, error)
	FindRole(ctx context.Context, id int64) (domain.Role, bool, error)
	InviteSchoolAdmin(ctx context.Context, request domain.SchoolAdminInvitationRequest) (domain.SchoolAdminInvitation, error)
	RegisterSchoolAccount(ctx context.Context, registration domain.SchoolAccountRegistration) (domain.CreatedAccount, error)
	EnsureSchoolIdentity(ctx context.Context, request domain.SchoolIdentityRequest) error
	AssignRole(ctx context.Context, tenantID, accountID, roleID int64) error
	ListSchoolAccounts(ctx context.Context, tenantID int64) ([]domain.SchoolAccount, error)
	ListOrganizationAccounts(ctx context.Context, organizationID int64) ([]domain.OrganizationAccount, error)
	ListAllAccounts(ctx context.Context) ([]domain.OrganizationAccount, error)
	// RevokeSchoolSessions deletes every refresh session of the school.
	RevokeSchoolSessions(ctx context.Context, tenantID int64) (int, error)
	// InvalidatePendingInvitations consumes every open invitation of the school.
	InvalidatePendingInvitations(ctx context.Context, tenantID int64) (int, error)
	DeactivateAccount(ctx context.Context, accountID int64) error
	// AnonymizeAccount replaces the account's e-mail for a deleted person.
	AnonymizeAccount(ctx context.Context, accountID int64, email string) error
}

// ProvisioningDevices is the Device Fleet part of provisioning. A missing
// device is found=false; write conflicts are domain.ErrDeviceIDTaken,
// domain.ErrDeviceAPIKeyTaken and domain.ErrDeviceReferenced.
type ProvisioningDevices interface {
	CountDevicesByTenant(ctx context.Context) (map[int64]int, error)
	ListDevicesByTenant(ctx context.Context, tenantIDs []int64) ([]domain.Device, error)
	// FindDeviceForUpdate reads and locks one live device of any school.
	FindDeviceForUpdate(ctx context.Context, id int64) (domain.Device, bool, error)
	CreateDevice(ctx context.Context, device domain.NewDevice) (domain.Device, error)
	// UpdateDevice replaces every writable column of the device.
	UpdateDevice(ctx context.Context, device domain.Device) error
	DeleteDevice(ctx context.Context, id int64) error
}

// ProvisioningPeople is the People Directory and School Membership part of
// provisioning. A missing person is found=false.
type ProvisioningPeople interface {
	CountPersonsByTenant(ctx context.Context) (map[int64]int, error)
	// CountChildQuotaByTenant is the Kontingentzahl of every school (#3568),
	// counted by School Membership's rule on today's Berlin calendar day.
	CountChildQuotaByTenant(ctx context.Context) (map[int64]int, error)
	ListPersons(ctx context.Context, tenantIDs []int64) ([]domain.PersonListing, error)
	// FindPerson reads one non-deleted person of any school.
	FindPerson(ctx context.Context, id int64) (domain.Person, bool, error)
	// FindStaff reads the staff record of the person, found=false when the
	// person is no staff member.
	FindStaff(ctx context.Context, personID int64) (domain.StaffMember, bool, error)
	UnlinkRFIDCard(ctx context.Context, personID int64) error
	UnlinkAccount(ctx context.Context, personID int64) error
	AnonymizeAndSoftDelete(ctx context.Context, personID int64) error
}

// ProvisioningPresence is the Student Presence part of provisioning.
type ProvisioningPresence interface {
	// ActiveDeviceSession returns the device's open kiosk session, nil when
	// there is none.
	ActiveDeviceSession(ctx context.Context, tenantID, deviceID int64) (*domain.DeviceSession, error)
	CountActiveSupervisions(ctx context.Context, tenantID, staffID int64) (int, error)
}

// ProvisioningCategories seeds the default activity categories of a new
// school. A category the school already has is skipped.
type ProvisioningCategories interface {
	SeedCategories(ctx context.Context, tenantID int64, categories []domain.ActivityCategory) error
}

// ProvisioningSettings resolves the per-school device online window.
type ProvisioningSettings interface {
	DeviceOnlineWindowMinutes(ctx context.Context, tenantID int64) (int, error)
}

// OperatorAudit appends to the platform operator audit ledger.
type OperatorAudit interface {
	RecordOperatorAction(ctx context.Context, entry domain.OperatorAuditEntry) error
}

// ProvisioningSecrets generates device API keys and username suffixes.
type ProvisioningSecrets interface {
	APIKey() (string, error)
	UsernameSuffix() string
}
