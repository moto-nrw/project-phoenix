package auth

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
)

// CleanupExpiredTokens removes expired authentication tokens
func (s *Service) CleanupExpiredTokens(ctx context.Context) (int, error) {
	count, err := s.repos.Token.DeleteExpiredTokens(ctx)
	if err != nil {
		return 0, &AuthError{Op: "cleanup expired tokens", Err: err}
	}
	return count, nil
}

// RevokeAllTokens revokes all tokens for an account
func (s *Service) RevokeAllTokens(ctx context.Context, accountID int) error {
	if err := s.repos.Token.DeleteByAccountID(ctx, int64(accountID)); err != nil {
		return &AuthError{Op: "revoke all tokens", Err: err}
	}
	return nil
}

// GetActiveTokens retrieves all active tokens for an account
func (s *Service) GetActiveTokens(ctx context.Context, accountID int) ([]*auth.Token, error) {
	filters := map[string]interface{}{
		"account_id": int64(accountID),
		"active":     true,
	}

	tokens, err := s.repos.Token.List(ctx, filters)
	if err != nil {
		return nil, &AuthError{Op: "get active tokens", Err: err}
	}
	return tokens, nil
}
