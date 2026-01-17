package auth

import (
	"context"

	jwx "github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
)

// ValidateToken validates an access token and returns the associated account
func (s *Service) ValidateToken(ctx context.Context, tokenString string) (*auth.Account, error) {
	// Parse and validate JWT token
	jwtToken, err := s.tokenProvider.Decode(tokenString)
	if err != nil {
		return nil, &AuthError{Op: "validate token", Err: ErrInvalidToken}
	}

	// Extract claims
	claims := extractClaims(jwtToken)

	// Parse claims into AppClaims
	var appClaims port.AppClaims
	err = appClaims.ParseClaims(claims)
	if err != nil {
		return nil, &AuthError{Op: "parse claims", Err: ErrInvalidToken}
	}

	// Get account by ID
	account, err := s.repos.Account.FindByID(ctx, int64(appClaims.ID))
	if err != nil {
		return nil, &AuthError{Op: opGetAccount, Err: ErrAccountNotFound}
	}

	// Ensure account is active
	if !account.Active {
		return nil, &AuthError{Op: "validate token", Err: ErrAccountInactive}
	}

	// Load roles and permissions if not already loaded
	s.ensureAccountRolesLoaded(ctx, account)
	s.ensureAccountPermissionsLoaded(ctx, account)

	return account, nil
}

// extractClaims extracts all claims from a jwt token into a map
func extractClaims(token jwx.Token) map[string]interface{} {
	claims := make(map[string]interface{})

	// Extract private claims
	for k, v := range token.PrivateClaims() {
		claims[k] = v
	}

	// Add registered claims if present
	if sub, ok := token.Get(jwx.SubjectKey); ok {
		claims[jwx.SubjectKey] = sub
	}
	if exp, ok := token.Get(jwx.ExpirationKey); ok {
		claims[jwx.ExpirationKey] = exp
	}

	return claims
}
