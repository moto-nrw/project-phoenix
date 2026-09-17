package platform

import (
	"context"
	"time"
)

// The operator identity rows and their refresh sessions are owned by the
// Identity & Access module (#2720, #3252); the retained flows in
// services/platform reach them through their own consumer-owned ports. The
// same holds for the operator MFA records (#2723).

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
	FindByDateRange(ctx context.Context, start, end time.Time, limit int) ([]*OperatorAuditLog, error)
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
