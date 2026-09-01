package platform

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// OperatorRepository defines operations for managing operators
type OperatorRepository interface {
	// Core CRUD operations
	Create(ctx context.Context, operator *Operator) error
	FindByID(ctx context.Context, id int64) (*Operator, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*Operator, error)
	FindByEmail(ctx context.Context, email string) (*Operator, error)
	Update(ctx context.Context, operator *Operator) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]*Operator, error)

	// Auth operations
	UpdateLastLogin(ctx context.Context, id int64) error

	// IncrementMFAAttempts atomically bumps mfa_attempts and applies the
	// lockout window once threshold is reached. Mirrors the auth.Account
	// version from #1430 review item #6 — prevents concurrent failed
	// verifies from collapsing into a single counted attempt.
	IncrementMFAAttempts(ctx context.Context, id int64, threshold int, lockoutDuration time.Duration) (OperatorMFAAttemptResult, error)
	// ResetMFAAttempts atomically clears mfa_attempts + mfa_locked_until
	// after a successful verify.
	ResetMFAAttempts(ctx context.Context, id int64) error
}

// OperatorMFAAttemptResult is the post-update snapshot returned by
// OperatorRepository.IncrementMFAAttempts. Mirrors auth.MFAAttemptResult.
type OperatorMFAAttemptResult struct {
	Attempts    int
	LockedUntil *time.Time
}

// OrganizationRepository defines operations for managing organizations.
type OrganizationRepository interface {
	Create(ctx context.Context, organization *Organization) error
	FindByID(ctx context.Context, id int64) (*Organization, error)
	FindByIDForShare(ctx context.Context, id int64) (*Organization, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*Organization, error)
	FindBySlug(ctx context.Context, slug string) (*Organization, error)
	List(ctx context.Context) ([]*Organization, error)
	Update(ctx context.Context, organization *Organization) error
	CountByIDs(ctx context.Context, ids []int64) (int, error)
	SoftDelete(ctx context.Context, id int64) error
	Restore(ctx context.Context, id int64) error
}

// AnnouncementRepository defines operations for managing announcements
type AnnouncementRepository interface {
	// Core CRUD operations
	Create(ctx context.Context, announcement *Announcement) error
	FindByID(ctx context.Context, id int64) (*Announcement, error)
	Update(ctx context.Context, announcement *Announcement) error
	Delete(ctx context.Context, id int64) error

	// Listing operations
	List(ctx context.Context, includeInactive bool) ([]*Announcement, error)
	// Publishing
	Publish(ctx context.Context, id int64) error
	Unpublish(ctx context.Context, id int64) error
}

// AnnouncementViewRepository defines operations for tracking announcement views
type AnnouncementViewRepository interface {
	// Mark as seen/dismissed
	MarkSeen(ctx context.Context, userID, announcementID int64) error
	MarkDismissed(ctx context.Context, userID, announcementID int64) error

	// Query unread announcements for a user scoped to the current session tenant/org
	GetUnreadForUser(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) ([]*Announcement, error)

	// Count unread announcements for a user scoped to the current session tenant/org
	CountUnread(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) (int, error)

	// Get view statistics for an announcement
	GetStats(ctx context.Context, announcementID int64) (*AnnouncementStats, error)

	// Get detailed view list for an announcement (who has seen/dismissed)
	GetViewDetails(ctx context.Context, announcementID int64) ([]*AnnouncementViewDetail, error)
}

// SchoolRepository defines operations for school (tenant) records.
type SchoolRepository interface {
	Create(ctx context.Context, school *School) error
	FindByID(ctx context.Context, id int64) (*School, error)
	FindByIDForShare(ctx context.Context, id int64) (*School, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*School, error)
	FindBySlug(ctx context.Context, slug string) (*School, error)
	FindByOrganizationAndSlug(ctx context.Context, organizationID int64, slug string) (*School, error)
	FindBySubdomain(ctx context.Context, subdomain string) (*School, error)
	List(ctx context.Context) ([]*School, error)
	// ListNonDeleted returns all non-deleted schools, regardless of whether
	// they currently admit users. Retention recovery uses this to clean files
	// for inactive tenants too.
	ListNonDeleted(ctx context.Context) ([]School, error)
	ListActive(ctx context.Context) ([]School, error)
	ListPublic(ctx context.Context) ([]School, error)
	FindActiveByAccountID(ctx context.Context, accountID int64) ([]School, error)
	Update(ctx context.Context, school *School) error
	SoftDelete(ctx context.Context, id int64) error
	Restore(ctx context.Context, id int64) error
	CountByIDs(ctx context.Context, ids []int64) (int, error)
	CountNonDeletedByOrganizationID(ctx context.Context, organizationID int64) (int, error)
}

// OperatorEmailChangeTokenRepository defines operations for email change verification tokens
type OperatorEmailChangeTokenRepository interface {
	Create(ctx context.Context, token *OperatorEmailChangeToken) error
	ConsumeByToken(ctx context.Context, tokenStr string) (*OperatorEmailChangeToken, error)
	InvalidateByOperatorID(ctx context.Context, operatorID int64) error
	UpdateDeliveryResult(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error
	CountRecentByOperatorID(ctx context.Context, operatorID int64, since time.Time) (int, error)
	InvalidateExpiredTokens(ctx context.Context) (int, error)
	DeleteStaleTokens(ctx context.Context) (int, error)
}

// OperatorRefreshTokenRepository persists revocable platform-operator refresh sessions.
type OperatorRefreshTokenRepository interface {
	Create(ctx context.Context, token *OperatorRefreshToken) error
	FindByTokenForUpdate(ctx context.Context, token string) (*OperatorRefreshToken, error)
	MarkRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error
	DeleteExpiredRotated(ctx context.Context, familyID string, now time.Time) error
	Delete(ctx context.Context, id any) error
	DeleteByOperatorIDReturning(ctx context.Context, operatorID int64) ([]*OperatorRefreshToken, error)
	DeleteByFamilyIDReturning(ctx context.Context, familyID string) ([]*OperatorRefreshToken, error)
	GetLatestTokenInFamily(ctx context.Context, familyID string) (*OperatorRefreshToken, error)
	DeleteExpired(ctx context.Context, now time.Time) (int, error)
}

// OperatorInvitationTokenRepository defines operations for operator invitation tokens
type OperatorInvitationTokenRepository interface {
	Create(ctx context.Context, token *OperatorInvitationToken) error
	FindByID(ctx context.Context, id int64) (*OperatorInvitationToken, error)
	FindValidByToken(ctx context.Context, tokenStr string) (*OperatorInvitationToken, error)
	ConsumeByToken(ctx context.Context, tokenStr string) (*OperatorInvitationToken, error)
	MarkAsUsed(ctx context.Context, id int64) (bool, error)
	ListPending(ctx context.Context) ([]*OperatorInvitationToken, error)
	InvalidateByEmail(ctx context.Context, email string) (int, error)
	ExtendExpiry(ctx context.Context, id int64, newExpiresAt time.Time) (bool, error)
	UpdateDeliveryResult(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error
	DeleteExpired(ctx context.Context) (int, error)
	CountRecentByCreatedBy(ctx context.Context, createdByID int64, since time.Time) (int, error)
}

// OperatorAuditLogRepository defines operations for the audit log
type OperatorAuditLogRepository interface {
	// Create a new audit log entry
	Create(ctx context.Context, entry *OperatorAuditLog) error

	// Query audit logs
	FindByOperatorID(ctx context.Context, operatorID int64, limit int) ([]*OperatorAuditLog, error)
	FindByDateRange(ctx context.Context, start, end time.Time, limit int) ([]*OperatorAuditLog, error)
}

// OperatorMFACredentialRepository persists per-operator MFA enrollment.
// Mirror of auth.MFACredentialRepository for the platform.operator_*
// schema.
type OperatorMFACredentialRepository interface {
	base.CRUDRepository[*OperatorMFACredential]
	FindByOperatorID(ctx context.Context, operatorID int64) (*OperatorMFACredential, error)
	UpdateLastUsedAt(ctx context.Context, id int64, when time.Time) error
	DeleteByOperatorID(ctx context.Context, operatorID int64) error
}

// OperatorMFAEmailChallengeRepository persists time-limited 6-digit codes
// for moto-Operators. Mirror of auth.MFAEmailChallengeRepository.
type OperatorMFAEmailChallengeRepository interface {
	Create(ctx context.Context, challenge *OperatorMFAEmailChallenge) error
	FindByID(ctx context.Context, id interface{}) (*OperatorMFAEmailChallenge, error)
	FindActiveByOperatorID(ctx context.Context, operatorID int64) (*OperatorMFAEmailChallenge, error)
	MarkActive(ctx context.Context, id int64) error
	MarkConsumed(ctx context.Context, id int64, consumedAt time.Time) error
	CountRecentByOperatorID(ctx context.Context, operatorID int64, since time.Time) (int, error)
	DeleteExpired(ctx context.Context) (int, error)
}

// OperatorMFATrustedDeviceRepository persists HMAC-signed trusted-device
// records for moto-Operators. Mirror of auth.MFATrustedDeviceRepository.
type OperatorMFATrustedDeviceRepository interface {
	Create(ctx context.Context, device *OperatorMFATrustedDevice) error
	FindActiveByOperatorIDAndTokenHash(ctx context.Context, operatorID int64, tokenHash string) (*OperatorMFATrustedDevice, error)
	ListActiveByOperatorID(ctx context.Context, operatorID int64) ([]*OperatorMFATrustedDevice, error)
	UpdateLastUsedAt(ctx context.Context, id int64, when time.Time) error
	Revoke(ctx context.Context, id int64, revokedAt time.Time) error
	RevokeAllByOperatorID(ctx context.Context, operatorID int64, revokedAt time.Time) error
	DeleteExpired(ctx context.Context) (int, error)
}

// OperatorPasskeyCredentialRepository persists WebAuthn credentials for moto operators.
type OperatorPasskeyCredentialRepository interface {
	Create(ctx context.Context, credential *OperatorPasskeyCredential) error
	FindActiveByOperatorID(ctx context.Context, operatorID int64) ([]*OperatorPasskeyCredential, error)
	FindActiveByCredentialIDAndUserHandle(ctx context.Context, credentialID, userHandle []byte) (*OperatorPasskeyCredential, error)
	UpdateAfterUse(ctx context.Context, id int64, credentialJSON []byte, usedAt time.Time) error
	Revoke(ctx context.Context, operatorID, id int64, revokedAt time.Time) error
}

// OperatorPasskeySessionRepository persists server-side WebAuthn ceremony state for operators.
type OperatorPasskeySessionRepository interface {
	Create(ctx context.Context, session *OperatorPasskeySession) error
	Consume(ctx context.Context, id, purpose string, now time.Time) (*OperatorPasskeySession, error)
	DeleteExpired(ctx context.Context, now time.Time) (int, error)
}
