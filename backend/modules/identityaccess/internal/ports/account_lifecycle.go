package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// AccountLifecycleStore is the persistence port over the identity-owned rows
// the lifecycle flows (#3225) read and write beyond the login store: the PIN
// columns of auth.accounts, the school's account listing, the direct
// permission grants, the school mapping deactivation, account e-mails and
// auth.accounts_parents. Every statement runs on the connection the caller's
// context carries.
type AccountLifecycleStore interface {
	FindPINAccount(ctx context.Context, id int64, forUpdate bool) (domain.PINAccount, bool, domain.OperationStats, error)
	// IncrementPINAttempts bumps the counter and applies lockedUntil in one
	// statement once the post-increment count reaches threshold.
	IncrementPINAttempts(ctx context.Context, id int64, threshold int, lockedUntil time.Time) (domain.OperationStats, error)
	ResetPINAttempts(ctx context.Context, id int64) (domain.OperationStats, error)
	UpdatePINHash(ctx context.Context, id int64, hash string) (domain.OperationStats, error)

	// ListTenantAccounts returns every account mapped to the school with its
	// role names at that school, ascending by account id.
	ListTenantAccounts(ctx context.Context, tenantID int64) ([]domain.TenantAccount, domain.OperationStats, error)
	ListAccountPermissionGrants(ctx context.Context, accountID, tenantID int64) ([]domain.PermissionGrant, domain.OperationStats, error)
	DeleteAccountPermissionGrants(ctx context.Context, accountID, tenantID int64) (int64, domain.OperationStats, error)
	// DeactivateTenantMapping keeps the row (with deactivated_at) so a later
	// re-invitation can reactivate it.
	DeactivateTenantMapping(ctx context.Context, accountID, tenantID int64) (domain.OperationStats, error)
	ListAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, domain.OperationStats, error)

	FindParentAccount(ctx context.Context, id int64) (domain.ParentAccount, bool, domain.OperationStats, error)
	FindParentAccountByEmail(ctx context.Context, email string) (domain.ParentAccount, bool, domain.OperationStats, error)
	FindParentAccountByUsername(ctx context.Context, username string) (domain.ParentAccount, bool, domain.OperationStats, error)
	InsertParentAccount(ctx context.Context, account domain.ParentAccount) (domain.ParentAccount, domain.OperationStats, error)
	// UpdateParentAccount writes e-mail, username, active flag and password
	// hash and reports whether the row existed.
	UpdateParentAccount(ctx context.Context, account domain.ParentAccount) (bool, domain.OperationStats, error)
	ListParentAccounts(ctx context.Context, filter domain.ParentAccountFilter) ([]domain.ParentAccount, domain.OperationStats, error)
}

// StaffDirectory is the consumer-owned port over the People Directory and
// School Membership facts the lifecycle flows read and, for the school
// identity chain, write: persons, staff and caregiver profiles of the tenant
// the context carries. Missing rows report found=false; every other failure
// is an error.
type StaffDirectory interface {
	FindStaff(ctx context.Context, staffID int64) (domain.StaffMember, bool, error)
	FindPerson(ctx context.Context, personID int64) (domain.PersonRecord, bool, error)
	FindPersonByAccount(ctx context.Context, accountID int64) (domain.PersonRecord, bool, error)
	FindPersonByTag(ctx context.Context, tagID string) (domain.PersonRecord, bool, error)
	// FindPersonNames resolves the display names of the accounts' persons at
	// the school in context, keyed by account id.
	FindPersonNames(ctx context.Context, accountIDs []int64) (map[int64]domain.PersonName, error)
	CreatePerson(ctx context.Context, person domain.PersonRecord) (int64, error)
	LinkPersonToAccount(ctx context.Context, personID, accountID int64) error
	LinkPersonToRFIDCard(ctx context.Context, personID int64, tagID string) error
	IsStudentPerson(ctx context.Context, personID int64) (bool, error)
	FindStaffByPerson(ctx context.Context, personID int64) (domain.StaffMember, bool, error)
	CreateStaff(ctx context.Context, tenantID, personID int64) (int64, error)
	FindCaregiverProfile(ctx context.Context, staffID int64) (domain.CaregiverProfile, bool, error)
	CreateCaregiverProfile(ctx context.Context, tenantID, staffID int64, position string) (int64, error)
}

// RolePolicy classifies roles for the school identity chain; the public
// package owns the decisions, the composition binds them.
type RolePolicy interface {
	RoleNeedsStaffRecord(role *domain.RoleFacts) bool
	RoleNeedsCaregiverProfile(role *domain.RoleFacts) bool
}

// PINHasher hashes and verifies staff PINs with the credential hash the
// accounts store.
type PINHasher interface {
	HashPIN(pin string) (string, error)
	VerifyPIN(pin, hash string) bool
}

// LockoutPolicy resolves the tenant's PIN lockout threshold and duration for
// the context; the domain defaults apply without an override.
type LockoutPolicy interface {
	PINLockout(ctx context.Context) (threshold int, duration time.Duration)
}

// PreviewAudit is the consumer-owned port over the Audit platform's staff
// preview evidence. The writes run on the caller's transaction; the lock is
// transaction-scoped.
type PreviewAudit interface {
	RecordStaffPreviewStart(ctx context.Context, event domain.StaffPreviewEvent) error
	// RecordStaffPreviewEndOnce reports whether THIS call wrote the row.
	RecordStaffPreviewEndOnce(ctx context.Context, event domain.StaffPreviewEvent) (bool, error)
	LockStaffPreview(ctx context.Context, adminAccountID int64, previewID string) error
	StaffPreviewEnded(ctx context.Context, adminAccountID int64, previewID string) (bool, error)
}

// AccessTokenCodec signs and parses access-only JWTs for the staff preview.
type AccessTokenCodec interface {
	IssueAccessToken(claims domain.SessionClaims) (string, error)
	// ParseAccessTokenAllowExpired verifies the signature but not the expiry.
	ParseAccessTokenAllowExpired(token string) (domain.SessionClaims, error)
	AccessExpiry() time.Duration
}

// AccountAdministration is the consumer-owned port over the retained role
// and account management flows staff offboarding drives (#2721): a role
// removal revokes the sessions it must, a deactivation records the durable
// account-wide revocation intent. Both join the caller's transaction.
type AccountAdministration interface {
	RemoveRoleFromAccount(ctx context.Context, accountID, roleID int64) error
	DeactivateAccount(ctx context.Context, accountID int64) error
}

// PasswordPolicy validates and hashes parent account passwords with the
// retained password helpers.
type PasswordPolicy interface {
	ValidatePasswordStrength(password string) error
	HashPassword(password string) (string, error)
}

// GuardianDirectory is the consumer-owned port over the People Directory's
// guardian profiles, student-guardian relationships and students the
// guardian relative access flows read and write. Reads apply the tenant in
// context; missing rows report found=false.
type GuardianDirectory interface {
	FindGuardianProfileByEmail(ctx context.Context, email string) (domain.GuardianProfile, bool, error)
	FindGuardianProfile(ctx context.Context, id int64) (domain.GuardianProfile, bool, error)
	FindGuardianProfiles(ctx context.Context, ids []int64) (map[int64]domain.GuardianProfile, error)
	CreateGuardianProfile(ctx context.Context, profile domain.GuardianProfile) (int64, error)
	DeleteGuardianProfile(ctx context.Context, id int64) error
	LinkGuardianProfileToAccount(ctx context.Context, profileID, accountID int64) error

	// FindStudentGuardianLinkForUpdate locks the relationship row for the
	// caller's transaction.
	FindStudentGuardianLinkForUpdate(ctx context.Context, studentID, guardianProfileID int64) (domain.StudentGuardianLink, bool, error)
	// LinkStudentGuardianIfAbsent inserts the relationship and reports
	// whether this call created it.
	LinkStudentGuardianIfAbsent(ctx context.Context, link domain.StudentGuardianLink) (bool, error)
	// PromoteStudentGuardianLink upgrades the relationship to legal guardian
	// with the server-derived parent-portal permission set (#2172).
	PromoteStudentGuardianLink(ctx context.Context, linkID int64) error
	ListStudentGuardianLinksByStudent(ctx context.Context, studentID int64) ([]domain.StudentGuardianLink, error)
	ListStudentGuardianLinksByProfile(ctx context.Context, guardianProfileID int64) ([]domain.StudentGuardianLink, error)
	DeleteStudentGuardianLink(ctx context.Context, linkID int64) error
	// GuardianRoleClass classifies a stored guardian role for the invite flow.
	GuardianRoleClass(role string) domain.GuardianRoleClass

	FindStudents(ctx context.Context, ids []int64) (map[int64]domain.Student, error)
	// LockStudent locks the child's row for the caller's transaction so a
	// concurrent payer assignment cannot slip past a removal.
	LockStudent(ctx context.Context, studentID int64) error
	FindPersonNamesByIDs(ctx context.Context, personIDs []int64) (map[int64]domain.PersonName, error)
}

// GuardianInvitationStore is the persistence port over
// auth.guardian_invitations and the account an acceptance creates, as the
// guardian invitation lifecycle and the relative access flows read and
// write them (#2722). Every statement runs on the connection the caller's
// context carries; the public validate and accept routes hold the
// administrative transaction.
type GuardianInvitationStore interface {
	FindGuardianInvitation(ctx context.Context, id int64) (domain.GuardianInvitation, bool, error)
	FindGuardianInvitationByToken(ctx context.Context, token string) (domain.GuardianInvitation, bool, error)
	ListGuardianInvitationsByProfile(ctx context.Context, guardianProfileID int64) ([]domain.GuardianInvitation, error)
	ListPendingGuardianApprovals(ctx context.Context) ([]domain.GuardianInvitation, error)
	InsertGuardianInvitation(ctx context.Context, invitation domain.GuardianInvitation) (domain.GuardianInvitation, error)
	UpdateGuardianInvitation(ctx context.Context, invitation domain.GuardianInvitation) error
	// AcceptGuardianInvitation stamps the acceptance and reports whether the
	// invitation was still unaccepted, so a token is spent exactly once.
	AcceptGuardianInvitation(ctx context.Context, id int64, acceptedAt time.Time) (bool, error)

	AccountProvisioning
}

// GuardianEnrollments is the consumer-owned port over the Enrollment
// module's pre-account requests: accepting an invitation claims the
// requests the guardian filed before they had an account (#2722).
type GuardianEnrollments interface {
	// ClaimGuardianEnrollments stamps the account onto every request of
	// that address and returns how many it claimed.
	ClaimGuardianEnrollments(ctx context.Context, accountID int64, email string) (int, error)
}

// GuardianInvitationDelivery is the consumer-owned port over the retained
// guardian invitation service's delivery seams (#2722): the token expiry the
// tenant configured, the school name for the mail and the outbox enqueue.
// EnqueueExistingAccountEmail tells an account holder who got access without
// a token invitation to sign in to the parents portal with their credentials.
type GuardianInvitationDelivery interface {
	InvitationExpiry(ctx context.Context) time.Duration
	SchoolName(ctx context.Context, tenantID int64) string
	EnqueueInvitationEmail(ctx context.Context, invitation domain.GuardianInvitation, profile domain.GuardianProfile, schoolName string)
	EnqueueExistingAccountEmail(ctx context.Context, profile domain.GuardianProfile, schoolName string)
}

// FinancialAudit is the consumer-owned port over the Audit platform's
// guardian financial ledger: removing a child's payer is a financial change.
type FinancialAudit interface {
	RecordPayerRemoved(ctx context.Context, guardianProfileID, studentID, actorAccountID int64) error
}
