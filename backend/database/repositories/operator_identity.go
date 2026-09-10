package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// IdentityAccess is the slice of the Identity & Access capability the legacy
// repository graph consumes: operator identity and refresh sessions (#2720)
// and the platform account facts the Care Plan parent reads need.
type IdentityAccess interface {
	identityaccess.GuardianAccessQuery
	identityaccess.OperatorAccess
}

// NewOperatorRepositories serves the retained operator repository contracts
// over the Identity & Access owner for compositions that do not build the
// legacy factory (session validation, focused tests).
func NewOperatorRepositories(identity identityaccess.OperatorAccess) (platform.OperatorRepository, platform.OperatorRefreshTokenRepository) {
	if identity == nil {
		panic("operator repositories: identity access capability is required")
	}
	return operatorRepository{identity: identity}, operatorRefreshTokenRepository{identity: identity}
}

// NewOperatorAuditLogRepository serves the retained operator audit-log
// contract over the Audit owner's platform ledger.
func NewOperatorAuditLogRepository(entries auditModels.OperatorAuditLogRepository) platform.OperatorAuditLogRepository {
	if entries == nil {
		panic("operator audit log repository: audit ledger is required")
	}
	return operatorAuditLogRepository{entries: entries}
}

// operatorRepository is the compatibility adapter behind
// platform.OperatorRepository. It keeps the retained contract: a missing row
// is (nil, nil), every failure is a DatabaseError, and validation runs on the
// retained model before the owner sees the value.
type operatorRepository struct{ identity identityaccess.OperatorAccess }

func (r operatorRepository) Create(ctx context.Context, operator *platform.Operator) error {
	if operator == nil {
		return fmt.Errorf("%s cannot be nil or zero value", "Operator")
	}
	if err := operator.Validate(); err != nil {
		return err
	}
	stored, err := r.identity.CreateOperator(ctx, identityOperator(operator))
	if err != nil {
		return authRepo.DatabaseError("create", err)
	}
	applyIdentityOperator(operator, stored)
	return nil
}

func (r operatorRepository) FindByID(ctx context.Context, id int64) (*platform.Operator, error) {
	operator, err := r.identity.FindOperator(ctx, id)
	return operatorResult(operator, err, "find operator by id")
}

func (r operatorRepository) FindByIDForUpdate(ctx context.Context, id int64) (*platform.Operator, error) {
	operator, err := r.identity.FindOperatorForUpdate(ctx, id)
	return operatorResult(operator, err, "find operator by id for update")
}

func (r operatorRepository) FindByEmail(ctx context.Context, email string) (*platform.Operator, error) {
	operator, err := r.identity.FindOperatorByEmail(ctx, email)
	return operatorResult(operator, err, "find operator by email")
}

func operatorResult(operator identityaccess.Operator, err error, op string) (*platform.Operator, error) {
	if errors.Is(err, identityaccess.ErrOperatorNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, authRepo.DatabaseError(op, err)
	}
	return operatorModel(operator), nil
}

func (r operatorRepository) Update(ctx context.Context, operator *platform.Operator) error {
	if operator == nil {
		return fmt.Errorf("%s cannot be nil or zero value", "Operator")
	}
	if err := operator.Validate(); err != nil {
		return err
	}
	stored, err := r.identity.UpdateOperator(ctx, identityOperator(operator))
	if err != nil {
		return authRepo.DatabaseError("update", err)
	}
	applyIdentityOperator(operator, stored)
	return nil
}

func (r operatorRepository) Delete(ctx context.Context, id int64) error {
	if err := r.identity.DeleteOperator(ctx, id); err != nil {
		return authRepo.DatabaseError("delete", err)
	}
	return nil
}

func (r operatorRepository) List(ctx context.Context) ([]*platform.Operator, error) {
	operators, err := r.identity.ListOperators(ctx)
	if err != nil {
		return nil, authRepo.DatabaseError("list operators", err)
	}
	result := make([]*platform.Operator, 0, len(operators))
	for _, operator := range operators {
		result = append(result, operatorModel(operator))
	}
	return result, nil
}

func (r operatorRepository) UpdateLastLogin(ctx context.Context, id int64) error {
	if err := r.identity.RecordOperatorLogin(ctx, id); err != nil {
		return authRepo.DatabaseError("update columns", err)
	}
	return nil
}

func (r operatorRepository) IncrementMFAAttempts(ctx context.Context, id int64, threshold int, lockoutDuration time.Duration) (platform.OperatorMFAAttemptResult, error) {
	attempts, err := r.identity.IncrementOperatorMFAAttempts(ctx, id, threshold, lockoutDuration)
	if err != nil {
		return platform.OperatorMFAAttemptResult{}, authRepo.DatabaseError("increment operator mfa attempts", err)
	}
	return platform.OperatorMFAAttemptResult{Attempts: attempts.Attempts, LockedUntil: attempts.LockedUntil}, nil
}

func (r operatorRepository) ResetMFAAttempts(ctx context.Context, id int64) error {
	if err := r.identity.ResetOperatorMFAAttempts(ctx, id); err != nil {
		return authRepo.DatabaseError("reset operator mfa attempts", err)
	}
	return nil
}

func identityOperator(operator *platform.Operator) identityaccess.Operator {
	return identityaccess.Operator{
		ID: operator.ID, Email: operator.Email, DisplayName: operator.DisplayName, PasswordHash: operator.PasswordHash, Active: operator.Active,
		LastLogin: operator.LastLogin, MFAAttempts: operator.MFAAttempts, MFALockedUntil: operator.MFALockedUntil,
		CreatedAt: operator.CreatedAt, UpdatedAt: operator.UpdatedAt,
	}
}

func applyIdentityOperator(dst *platform.Operator, src identityaccess.Operator) {
	dst.ID = src.ID
	dst.Email = src.Email
	dst.DisplayName = src.DisplayName
	dst.PasswordHash = src.PasswordHash
	dst.Active = src.Active
	dst.LastLogin = src.LastLogin
	dst.MFAAttempts = src.MFAAttempts
	dst.MFALockedUntil = src.MFALockedUntil
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
}

func operatorModel(src identityaccess.Operator) *platform.Operator {
	operator := &platform.Operator{}
	applyIdentityOperator(operator, src)
	return operator
}

// operatorRefreshTokenRepository is the compatibility adapter behind
// platform.OperatorRefreshTokenRepository over the owner's sessions.
type operatorRefreshTokenRepository struct{ identity identityaccess.OperatorAccess }

func (r operatorRefreshTokenRepository) Create(ctx context.Context, token *platform.OperatorRefreshToken) error {
	if token == nil {
		return fmt.Errorf("%s cannot be nil or zero value", "OperatorRefreshToken")
	}
	if err := token.Validate(); err != nil {
		return err
	}
	stored, err := r.identity.CreateOperatorSession(ctx, identitySession(token))
	if err != nil {
		return authRepo.DatabaseError("create", err)
	}
	applyIdentitySession(token, stored)
	return nil
}

func (r operatorRefreshTokenRepository) FindByTokenForUpdate(ctx context.Context, tokenValue string) (*platform.OperatorRefreshToken, error) {
	session, err := r.identity.FindOperatorSessionForUpdate(ctx, tokenValue)
	return sessionResult(session, err, "find operator refresh token for update")
}

func (r operatorRefreshTokenRepository) GetLatestTokenInFamily(ctx context.Context, familyID string) (*platform.OperatorRefreshToken, error) {
	session, err := r.identity.LatestOperatorSessionInFamily(ctx, familyID)
	return sessionResult(session, err, "get latest operator refresh token in family")
}

func sessionResult(session identityaccess.OperatorSession, err error, op string) (*platform.OperatorRefreshToken, error) {
	if errors.Is(err, identityaccess.ErrOperatorSessionNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, authRepo.DatabaseError(op, err)
	}
	return sessionModel(session), nil
}

func (r operatorRefreshTokenRepository) MarkRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error {
	if err := r.identity.MarkOperatorSessionRotated(ctx, id, replacementToken, recoveryProofHash, rotatedAt); err != nil {
		return authRepo.DatabaseError("mark operator refresh token rotated", err)
	}
	return nil
}

func (r operatorRefreshTokenRepository) DeleteExpiredRotated(ctx context.Context, familyID string, now time.Time) error {
	if err := r.identity.DeleteExpiredRotatedOperatorSessions(ctx, familyID, now); err != nil {
		return authRepo.DatabaseError("delete expired rotated operator refresh tokens", err)
	}
	return nil
}

func (r operatorRefreshTokenRepository) Delete(ctx context.Context, id any) error {
	sessionID, ok := sessionIdentifier(id)
	if !ok {
		return authRepo.DatabaseError("delete operator refresh token", fmt.Errorf("unsupported id %T", id))
	}
	if err := r.identity.DeleteOperatorSession(ctx, sessionID); err != nil {
		return authRepo.DatabaseError("delete operator refresh token", err)
	}
	return nil
}

func sessionIdentifier(id any) (int64, bool) {
	switch value := id.(type) {
	case int64:
		return value, true
	case int:
		return int64(value), true
	case int32:
		return int64(value), true
	default:
		return 0, false
	}
}

func (r operatorRefreshTokenRepository) DeleteByOperatorIDReturning(ctx context.Context, operatorID int64) ([]*platform.OperatorRefreshToken, error) {
	sessions, err := r.identity.RevokeOperatorSessions(ctx, operatorID)
	if err != nil {
		return nil, authRepo.DatabaseError("delete and return operator refresh tokens", err)
	}
	return sessionModels(sessions), nil
}

func (r operatorRefreshTokenRepository) DeleteByFamilyIDReturning(ctx context.Context, familyID string) ([]*platform.OperatorRefreshToken, error) {
	sessions, err := r.identity.RevokeOperatorSessionFamily(ctx, familyID)
	if err != nil {
		return nil, authRepo.DatabaseError("delete and return operator refresh-token family", err)
	}
	return sessionModels(sessions), nil
}

func (r operatorRefreshTokenRepository) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	deleted, err := r.identity.DeleteExpiredOperatorSessions(ctx, now)
	if err != nil {
		return 0, authRepo.DatabaseError("delete expired operator refresh tokens", err)
	}
	return deleted, nil
}

func identitySession(token *platform.OperatorRefreshToken) identityaccess.OperatorSession {
	return identityaccess.OperatorSession{
		ID: token.ID, OperatorID: token.OperatorID, Token: token.Token, Expiry: token.Expiry, FamilyID: token.FamilyID, Generation: token.Generation,
		RotatedAt: token.RotatedAt, ReplacementToken: token.ReplacementToken, RecoveryProofHash: token.RecoveryProofHash,
		CreatedAt: token.CreatedAt, UpdatedAt: token.UpdatedAt,
	}
}

func applyIdentitySession(dst *platform.OperatorRefreshToken, src identityaccess.OperatorSession) {
	dst.ID = src.ID
	dst.OperatorID = src.OperatorID
	dst.Token = src.Token
	dst.Expiry = src.Expiry
	dst.FamilyID = src.FamilyID
	dst.Generation = src.Generation
	dst.RotatedAt = src.RotatedAt
	dst.ReplacementToken = src.ReplacementToken
	dst.RecoveryProofHash = src.RecoveryProofHash
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
}

func sessionModel(src identityaccess.OperatorSession) *platform.OperatorRefreshToken {
	token := &platform.OperatorRefreshToken{}
	applyIdentitySession(token, src)
	return token
}

func sessionModels(sessions []identityaccess.OperatorSession) []*platform.OperatorRefreshToken {
	result := make([]*platform.OperatorRefreshToken, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, sessionModel(session))
	}
	return result
}

// operatorAuditLogRepository is the compatibility adapter behind
// platform.OperatorAuditLogRepository over the Audit owner's ledger.
type operatorAuditLogRepository struct {
	entries auditModels.OperatorAuditLogRepository
}

func (r operatorAuditLogRepository) Create(ctx context.Context, entry *platform.OperatorAuditLog) error {
	if entry == nil {
		return authRepo.DatabaseError("create audit log entry", errors.New("audit log entry is required"))
	}
	stored := auditEntry(entry)
	if err := r.entries.Create(ctx, stored); err != nil {
		return authRepo.DatabaseError("create audit log entry", err)
	}
	entry.ID = stored.ID
	entry.CreatedAt = stored.CreatedAt
	return nil
}

func (r operatorAuditLogRepository) FindByOperatorID(ctx context.Context, operatorID int64, limit int) ([]*platform.OperatorAuditLog, error) {
	entries, err := r.entries.FindByOperatorID(ctx, operatorID, limit)
	if err != nil {
		return nil, authRepo.DatabaseError("find audit logs by operator id", err)
	}
	return auditLogModels(entries), nil
}

func (r operatorAuditLogRepository) FindByDateRange(ctx context.Context, start, end time.Time, limit int) ([]*platform.OperatorAuditLog, error) {
	entries, err := r.entries.FindByDateRange(ctx, start, end, limit)
	if err != nil {
		return nil, authRepo.DatabaseError("find audit logs by date range", err)
	}
	return auditLogModels(entries), nil
}

func auditEntry(entry *platform.OperatorAuditLog) *auditModels.OperatorAuditEntry {
	return &auditModels.OperatorAuditEntry{
		ID: entry.ID, OperatorID: entry.OperatorID, Action: entry.Action, ResourceType: entry.ResourceType,
		ResourceID: entry.ResourceID, Changes: entry.Changes, RequestIP: entry.RequestIP, CreatedAt: entry.CreatedAt,
	}
}

func auditLogModels(entries []*auditModels.OperatorAuditEntry) []*platform.OperatorAuditLog {
	result := make([]*platform.OperatorAuditLog, 0, len(entries))
	for _, entry := range entries {
		result = append(result, &platform.OperatorAuditLog{
			ID: entry.ID, OperatorID: entry.OperatorID, Action: entry.Action, ResourceType: entry.ResourceType,
			ResourceID: entry.ResourceID, Changes: entry.Changes, RequestIP: entry.RequestIP, CreatedAt: entry.CreatedAt,
		})
	}
	return result
}

// newOperatorAuditLog binds the Audit-owned operator ledger for the factory.
func newOperatorAuditLog(runtime auditRepo.Runtime) platform.OperatorAuditLogRepository {
	return NewOperatorAuditLogRepository(auditRepo.NewOperatorAuditLogRepository(runtime))
}
