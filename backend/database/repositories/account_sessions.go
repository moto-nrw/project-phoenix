package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/uptrace/bun"
)

// accountSessionRepository is the compatibility adapter behind
// authModels.TokenRepository. Identity & Access owns auth.tokens (#2720):
// tenant, parent and school login, refresh, switching, logout, session
// validation and revocation reach the table only through its public
// account-session capability. The adapter keeps the retained contract the
// auth service still consumes: every failure is a DatabaseError, a missing
// row on the lookups satisfies modelBase.IsNoRows, validation runs on the
// retained model, and the identity and timestamps are written back into the
// caller's value.
type accountSessionRepository struct {
	identity identityaccess.AccountSessionAccess
}

// NewTokenRepository composes the retained token contract over the Identity
// & Access owner for compositions that do not build the whole factory.
func NewTokenRepository(db *bun.DB) authModels.TokenRepository {
	return accountSessionRepository{identity: newIdentityAccess(db, nil)}
}

func (r accountSessionRepository) Create(ctx context.Context, token *authModels.Token) error {
	if token == nil {
		return fmt.Errorf("token cannot be nil")
	}
	if token.PortalScope == "" {
		token.PortalScope = authModels.PortalScopeUnknown
	}
	if err := token.Validate(); err != nil {
		return err
	}
	stored, err := r.identity.CreateAccountSession(ctx, identitySessionFromToken(token))
	if err != nil {
		return authRepo.DatabaseError("create", err)
	}
	applyAccountSession(token, stored)
	return nil
}

func (r accountSessionRepository) Delete(ctx context.Context, id any) error {
	sessionID, ok := int64Value(id)
	if !ok {
		return authRepo.DatabaseError("delete", fmt.Errorf("unsupported token id %T", id))
	}
	if err := r.identity.DeleteAccountSession(ctx, sessionID); err != nil {
		return authRepo.DatabaseError("delete", err)
	}
	return nil
}

// int64Value coerces the integer shapes the retained contract accepts for
// identifiers (`id any` on Delete, filter values on List).
func int64Value(id any) (int64, bool) {
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

// List serves the retained filter map: account_id, family_id, mobile, active
// and expired. Any other key is refused instead of silently ignored, because
// an ignored filter would widen the result set.
func (r accountSessionRepository) List(ctx context.Context, filters map[string]any) ([]*authModels.Token, error) {
	var filter identityaccess.AccountSessionFilter
	for field, value := range filters {
		if value == nil {
			continue
		}
		if err := applyTokenFilter(&filter, field, value); err != nil {
			return nil, authRepo.DatabaseError("list", err)
		}
	}
	sessions, err := r.identity.ListAccountSessions(ctx, filter)
	if err != nil {
		return nil, authRepo.DatabaseError("list", err)
	}
	return tokensFromIdentitySessions(sessions), nil
}

func applyTokenFilter(filter *identityaccess.AccountSessionFilter, field string, value any) error {
	switch field {
	case "account_id":
		accountID, ok := int64Value(value)
		if !ok {
			return fmt.Errorf("unsupported token filter value %T for account_id", value)
		}
		filter.AccountID = accountID
	case "family_id":
		familyID, ok := value.(string)
		if !ok {
			return fmt.Errorf("unsupported token filter value %T for family_id", value)
		}
		filter.FamilyID = familyID
	case "mobile":
		mobile, ok := value.(bool)
		if !ok {
			return fmt.Errorf("unsupported token filter value %T for mobile", value)
		}
		filter.Mobile = &mobile
	case "active":
		if active, ok := value.(bool); ok && active {
			if filter.Liveness == identityaccess.AccountSessionsExpired {
				return fmt.Errorf("conflicting token filters active and expired")
			}
			filter.Liveness = identityaccess.AccountSessionsLive
		}
	case "expired":
		if expired, ok := value.(bool); ok && expired {
			if filter.Liveness == identityaccess.AccountSessionsLive {
				return fmt.Errorf("conflicting token filters active and expired")
			}
			filter.Liveness = identityaccess.AccountSessionsExpired
		}
	default:
		return fmt.Errorf("unsupported token filter %q", field)
	}
	return nil
}

func (r accountSessionRepository) FindByToken(ctx context.Context, token string) (*authModels.Token, error) {
	session, err := r.identity.FindAccountSession(ctx, token)
	return tokenResult(session, err, "find by token")
}

func (r accountSessionRepository) FindByTokenForUpdate(ctx context.Context, token string) (*authModels.Token, error) {
	session, err := r.identity.FindAccountSessionForUpdate(ctx, token)
	return tokenResult(session, err, "find by token for update")
}

func tokenResult(session identityaccess.AccountSession, err error, op string) (*authModels.Token, error) {
	if errors.Is(err, identityaccess.ErrAccountSessionNotFound) {
		return nil, authRepo.NotFoundError(op, err)
	}
	if err != nil {
		return nil, authRepo.DatabaseError(op, err)
	}
	return tokenFromIdentitySession(session), nil
}

func (r accountSessionRepository) MarkRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error {
	err := r.identity.MarkAccountSessionRotated(ctx, id, replacementToken, recoveryProofHash, rotatedAt)
	if errors.Is(err, identityaccess.ErrAccountSessionRotated) {
		return authRepo.DatabaseError("mark refresh token rotated", fmt.Errorf("refresh token was already rotated or not found: %w", err))
	}
	if err != nil {
		return authRepo.DatabaseError("mark refresh token rotated", err)
	}
	return nil
}

func (r accountSessionRepository) DeleteExpiredRotatedForAccount(ctx context.Context, accountID int64, now time.Time) error {
	if err := r.identity.DeleteExpiredRotatedAccountSessions(ctx, accountID, now); err != nil {
		return authRepo.DatabaseError("delete expired rotated account refresh tokens", err)
	}
	return nil
}

func (r accountSessionRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*authModels.Token, error) {
	sessions, err := r.identity.ListAccountSessions(ctx, identityaccess.AccountSessionFilter{AccountID: accountID})
	if err != nil {
		return nil, authRepo.DatabaseError("find by account ID", err)
	}
	return tokensFromIdentitySessions(sessions), nil
}

func (r accountSessionRepository) CountExpiredTokens(ctx context.Context) (int, error) {
	count, err := r.identity.CountExpiredAccountSessions(ctx)
	if err != nil {
		return 0, authRepo.DatabaseError("count expired tokens", err)
	}
	return count, nil
}

func (r accountSessionRepository) DeleteExpiredTokens(ctx context.Context) (int, error) {
	deleted, err := r.identity.DeleteExpiredAccountSessions(ctx)
	if err != nil {
		return 0, authRepo.DatabaseError("delete expired tokens", err)
	}
	return deleted, nil
}

func (r accountSessionRepository) ListInactiveAccountIDsWithLiveTokens(ctx context.Context) ([]int64, error) {
	ids, err := r.identity.ListInactiveAccountIDsWithLiveSessions(ctx)
	if err != nil {
		return nil, authRepo.DatabaseError("list inactive accounts with live tokens", err)
	}
	return ids, nil
}

func (r accountSessionRepository) HasLiveTokensCreatedAfter(ctx context.Context, accountID int64, since time.Time) (bool, error) {
	exists, err := r.identity.HasLiveAccountSessionsCreatedAfter(ctx, accountID, since)
	if err != nil {
		return false, authRepo.DatabaseError("check live tokens created after", err)
	}
	return exists, nil
}

func (r accountSessionRepository) DeleteByAccountIDReturning(ctx context.Context, accountID int64) ([]*authModels.Token, error) {
	sessions, err := r.identity.RevokeAccountSessionsInTenant(ctx, accountID)
	if errors.Is(err, identityaccess.ErrTenantRequired) {
		return nil, authRepo.DatabaseError("delete and return by account ID", fmt.Errorf("tenant-scoped token delete requires tenant_id: %w", err))
	}
	if err != nil {
		return nil, authRepo.DatabaseError("delete and return by account ID", err)
	}
	return tokensFromIdentitySessions(sessions), nil
}

func (r accountSessionRepository) DeleteAllByAccountIDReturning(ctx context.Context, accountID int64) ([]*authModels.Token, error) {
	sessions, err := r.identity.RevokeAllAccountSessions(ctx, accountID)
	if err != nil {
		return nil, authRepo.DatabaseError("delete and return by account ID", err)
	}
	return tokensFromIdentitySessions(sessions), nil
}

func (r accountSessionRepository) DeleteByAccountIDCreatedAtOrBeforeReturning(ctx context.Context, accountID int64, cutoff time.Time) ([]*authModels.Token, error) {
	sessions, err := r.identity.RevokeAccountSessionsCreatedAtOrBefore(ctx, accountID, cutoff)
	if err != nil {
		return nil, authRepo.DatabaseError("delete and return by account ID", err)
	}
	return tokensFromIdentitySessions(sessions), nil
}

func (r accountSessionRepository) CleanupOldTokensForAccountReturning(ctx context.Context, accountID int64, portalScope string, keepCount int) ([]*authModels.Token, error) {
	sessions, err := r.identity.EnforceAccountSessionCap(ctx, accountID, portalScope, keepCount)
	if err != nil {
		return nil, authRepo.DatabaseError("delete and return old tokens", err)
	}
	if len(sessions) == 0 {
		return nil, nil
	}
	return tokensFromIdentitySessions(sessions), nil
}

func (r accountSessionRepository) DeleteByTenantIDReturning(ctx context.Context, tenantID int64) ([]*authModels.Token, error) {
	sessions, err := r.identity.RevokeTenantAccountSessions(ctx, tenantID)
	if err != nil {
		return nil, authRepo.DatabaseError("delete and return tokens by tenant ID", err)
	}
	return tokensFromIdentitySessions(sessions), nil
}

func (r accountSessionRepository) DeleteByFamilyIDReturning(ctx context.Context, familyID string) ([]*authModels.Token, error) {
	sessions, err := r.identity.RevokeAccountSessionFamily(ctx, familyID)
	if err != nil {
		return nil, authRepo.DatabaseError("delete and return tokens by family ID", err)
	}
	return tokensFromIdentitySessions(sessions), nil
}

func (r accountSessionRepository) RetireFamily(ctx context.Context, accountID int64, familyID string, expiry time.Time) error {
	if err := r.identity.RetireAccountSessionFamily(ctx, accountID, familyID, expiry); err != nil {
		return authRepo.DatabaseError("retire refresh-token family", err)
	}
	return nil
}

func (r accountSessionRepository) GetLatestTokenInFamily(ctx context.Context, familyID string) (*authModels.Token, error) {
	session, err := r.identity.LatestAccountSessionInFamily(ctx, familyID)
	if errors.Is(err, identityaccess.ErrAccountSessionNotFound) {
		return nil, authRepo.DatabaseError("get latest token in family", fmt.Errorf("token not found: %w", err))
	}
	if err != nil {
		return nil, authRepo.DatabaseError("get latest token in family", err)
	}
	return tokenFromIdentitySession(session), nil
}

func identitySessionFromToken(token *authModels.Token) identityaccess.AccountSession {
	return identityaccess.AccountSession{
		ID: token.ID, TenantID: token.TenantID, AccountID: token.AccountID, Token: token.Token, Expiry: token.Expiry, Mobile: token.Mobile,
		Identifier: token.Identifier, PortalScope: token.PortalScope, FamilyID: token.FamilyID, FamilyExpiryCap: token.FamilyExpiryCap,
		Generation: token.Generation, RotatedAt: token.RotatedAt, ReplacementToken: token.ReplacementToken, RecoveryProofHash: token.RecoveryProofHash,
		CreatedAt: token.CreatedAt, UpdatedAt: token.UpdatedAt,
	}
}

func applyAccountSession(dst *authModels.Token, src identityaccess.AccountSession) {
	dst.ID = src.ID
	dst.TenantID = src.TenantID
	dst.AccountID = src.AccountID
	dst.Token = src.Token
	dst.Expiry = src.Expiry
	dst.Mobile = src.Mobile
	dst.Identifier = src.Identifier
	dst.PortalScope = src.PortalScope
	dst.FamilyID = src.FamilyID
	dst.FamilyExpiryCap = src.FamilyExpiryCap
	dst.Generation = src.Generation
	dst.RotatedAt = src.RotatedAt
	dst.ReplacementToken = src.ReplacementToken
	dst.RecoveryProofHash = src.RecoveryProofHash
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
}

func tokenFromIdentitySession(src identityaccess.AccountSession) *authModels.Token {
	token := &authModels.Token{}
	applyAccountSession(token, src)
	return token
}

func tokensFromIdentitySessions(sessions []identityaccess.AccountSession) []*authModels.Token {
	result := make([]*authModels.Token, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, tokenFromIdentitySession(session))
	}
	return result
}
