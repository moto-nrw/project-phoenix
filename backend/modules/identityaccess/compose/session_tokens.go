package compose

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// JWTSigner is the signing primitive supplied by composition. Identity owns the
// claim schema; the primitive verifies signatures without deciding token use.
type JWTSigner interface {
	Encode(map[string]any) (jwt.Token, string, error)
	Decode(string) (jwt.Token, error)
}

// SignedIdentityTokens serves the session and MFA challenge ports with one signer.
type SignedIdentityTokens interface {
	TokenCodec
	MFAChallengeCodec
}

type nativeSessionCodec struct {
	signer        JWTSigner
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

func NewSessionTokenCodec(signer JWTSigner, accessExpiry, refreshExpiry time.Duration) (SignedIdentityTokens, error) {
	if signer == nil {
		return nil, errors.New("session token signer is required")
	}
	return &nativeSessionCodec{signer: signer, accessExpiry: accessExpiry, refreshExpiry: refreshExpiry}, nil
}

func (c *nativeSessionCodec) AccessExpiry() time.Duration  { return c.accessExpiry }
func (c *nativeSessionCodec) RefreshExpiry() time.Duration { return c.refreshExpiry }

func (c *nativeSessionCodec) IssueTokenPair(access identityaccess.SessionClaims, refresh identityaccess.RefreshClaims) (string, string, error) {
	accessToken, err := c.IssueAccessToken(access)
	if err != nil {
		return "", "", err
	}
	now := time.Now()
	expiry := refresh.ExpiresAt
	if expiry <= 0 {
		expiry = now.Add(c.refreshExpiry).Unix()
	}
	claims := map[string]any{"iat": now.Unix(), "exp": expiry}
	put(claims, "id", refresh.AccountID, refresh.AccountID != 0)
	put(claims, "token", refresh.Token, refresh.Token != "")
	put(claims, "tenant_id", refresh.TenantID, refresh.TenantID != 0)
	put(claims, "scope", refresh.Scope, refresh.Scope != "")
	refreshToken, err := c.sign(claims)
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

func (c *nativeSessionCodec) IssueAccessToken(claims identityaccess.SessionClaims) (string, error) {
	now := time.Now()
	wire := map[string]any{
		"roles": claims.Roles, "permissions": claims.Permissions,
		"iat": now.Unix(), "exp": now.Add(c.accessExpiry).Unix(),
	}
	put(wire, "id", claims.AccountID, claims.AccountID != 0)
	put(wire, "sub", claims.Email, claims.Email != "")
	put(wire, "username", claims.Username, claims.Username != "")
	put(wire, "first_name", claims.FirstName, claims.FirstName != "")
	put(wire, "last_name", claims.LastName, claims.LastName != "")
	put(wire, "is_admin", claims.IsAdmin, claims.IsAdmin)
	put(wire, "scope", claims.Scope, claims.Scope != "")
	put(wire, "tenant_id", claims.TenantID, claims.TenantID != 0)
	put(wire, "org_id", claims.OrgID, claims.OrgID != 0)
	put(wire, "family_id", claims.FamilyID, claims.FamilyID != "")
	put(wire, "read_only", claims.ReadOnly, claims.ReadOnly)
	put(wire, "acting_admin_id", claims.ActingAdminID, claims.ActingAdminID != 0)
	put(wire, "preview_id", claims.PreviewID, claims.PreviewID != "")
	return c.sign(wire)
}

func (c *nativeSessionCodec) IssueMFAEnrollmentToken(accountID, tenantID int64, scope string, ttl time.Duration) (string, error) {
	if scope != "school" && scope != "platform" {
		scope = "tenant"
	}
	now := time.Now()
	wire := map[string]any{"account_id": accountID, "scope": scope, "mfa_enrollment_pending": true, "iat": now.Unix(), "exp": now.Add(ttl).Unix()}
	put(wire, "tenant_id", tenantID, tenantID != 0)
	return c.sign(wire)
}

func (c *nativeSessionCodec) sign(claims map[string]any) (string, error) {
	_, token, err := c.signer.Encode(claims)
	return token, err
}

func (c *nativeSessionCodec) IssueChallengeToken(claims identityaccess.MFAChallengeClaims, ttl time.Duration) (string, error) {
	now := time.Now()
	wire := map[string]any{"account_id": claims.AccountID, "mfa_pending": true, "iat": now.Unix(), "exp": now.Add(ttl).Unix()}
	put(wire, "scope", claims.Scope, claims.Scope != "")
	put(wire, "tenant_id", claims.TenantID, claims.TenantID != 0)
	put(wire, "challenge_id", claims.ChallengeID, claims.ChallengeID != 0)
	return c.sign(wire)
}

func (c *nativeSessionCodec) ParseChallengeToken(token string) (identityaccess.MFAChallengeClaims, error) {
	wire, err := c.decode(token)
	if err != nil {
		return identityaccess.MFAChallengeClaims{}, err
	}
	claims, err := domain.DecodeMFAChallengeToken(wire, time.Now())
	return identityaccess.MFAChallengeClaims(claims), err
}

func put(claims map[string]any, key string, value any, include bool) {
	if include {
		claims[key] = value
	}
}

// decode verifies the signature, then normalizes registered and custom claims
// through JSON. Expiry policy belongs to the particular token operation below.
func (c *nativeSessionCodec) decode(token string) (map[string]any, error) {
	verified, err := c.signer.Decode(token)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(verified)
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := json.Unmarshal(encoded, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func (c *nativeSessionCodec) ParseAccessToken(token string) (identityaccess.SessionClaims, error) {
	claims, err := c.ParseAccessTokenAllowExpired(token)
	if err != nil {
		return identityaccess.SessionClaims{}, err
	}
	if claims.ExpiresAt > 0 && claims.ExpiresAt < time.Now().Unix() {
		return identityaccess.SessionClaims{}, errors.New("access token expired")
	}
	return claims, nil
}

func (c *nativeSessionCodec) ParseAccessTokenAllowExpired(token string) (identityaccess.SessionClaims, error) {
	wire, err := c.decode(token)
	if err != nil {
		return identityaccess.SessionClaims{}, err
	}
	claims, err := domain.DecodeSessionToken(wire)
	return identityaccess.SessionClaims(claims), err
}

// ParseRefreshToken verifies signatures; persisted sessions own expiry and revocation.
func (c *nativeSessionCodec) ParseRefreshToken(token string) (identityaccess.RefreshClaims, error) {
	wire, err := c.decode(token)
	if err != nil {
		return identityaccess.RefreshClaims{}, err
	}
	claims, err := domain.DecodeRefreshToken(wire)
	return identityaccess.RefreshClaims(claims), err
}
