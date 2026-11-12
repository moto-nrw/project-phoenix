package application

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/tokenprimitives"
)

// JWTSigner is the signing primitive supplied by composition. Identity owns the
// claim schema; the primitive verifies signatures without deciding token use.
type JWTSigner interface {
	Encode(map[string]any) (jwt.Token, string, error)
	Decode(string) (jwt.Token, error)
}

// NewSessionCodec binds the session, MFA challenge and invitation owner
// tokens to one signer.
func NewSessionCodec(signer JWTSigner, accessExpiry, refreshExpiry time.Duration) *SessionCodec {
	return &SessionCodec{signer: signer, accessExpiry: accessExpiry, refreshExpiry: refreshExpiry}
}

// SessionCodec signs and parses the identity tokens with one signer. The
// claim schema is tokenprimitives' wire codec.
type SessionCodec struct {
	signer        JWTSigner
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

func (c *SessionCodec) AccessExpiry() time.Duration { return c.accessExpiry }

func (c *SessionCodec) RefreshExpiry() time.Duration { return c.refreshExpiry }

// AccountOfAccessToken accepts only a live, completed session as ownership proof.
func (c *SessionCodec) AccountOfAccessToken(token string) (int64, error) {
	if token == "" {
		return 0, nil
	}
	claims, err := c.ParseAccessToken(token)
	if err != nil || !domain.SessionProvesInvitationOwnership(domain.SessionClaims(claims), time.Now()) {
		return 0, nil
	}
	return claims.AccountID, nil
}

func (c *SessionCodec) IssueTokenPair(access identityaccess.SessionClaims, refresh identityaccess.RefreshClaims) (string, string, error) {
	accessToken, err := c.IssueAccessToken(access)
	if err != nil {
		return "", "", err
	}
	now := time.Now()
	refreshToken, err := c.sign(tokenprimitives.EncodeRefresh(tokenprimitives.RefreshClaims(refresh), now, now.Add(c.refreshExpiry)))
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

func (c *SessionCodec) IssueAccessToken(claims identityaccess.SessionClaims) (string, error) {
	now := time.Now()
	return c.sign(tokenprimitives.EncodeAccess(tokenprimitives.AccessClaims(claims), now, now.Add(c.accessExpiry)))
}

func (c *SessionCodec) IssueMFAEnrollmentToken(accountID, tenantID int64, scope string, ttl time.Duration, credentialGeneration int64) (string, error) {
	wire, err := tokenprimitives.EncodeEnrollment(tokenprimitives.EnrollmentClaims{AccountID: accountID, Scope: scope, TenantID: tenantID, CredentialGeneration: credentialGeneration}, time.Now(), ttl)
	if err != nil {
		return "", err
	}
	return c.sign(wire)
}

// sign signs the JSON claims of a tokenprimitives wire value.
func (c *SessionCodec) sign(wire any) (string, error) {
	encoded, err := json.Marshal(wire)
	if err != nil {
		return "", err
	}
	var claims map[string]any
	if err := json.Unmarshal(encoded, &claims); err != nil {
		return "", err
	}
	_, token, err := c.signer.Encode(claims)
	return token, err
}

func (c *SessionCodec) IssueChallengeToken(claims identityaccess.MFAChallengeClaims, ttl time.Duration) (string, error) {
	return c.sign(tokenprimitives.EncodeChallenge(tokenprimitives.ChallengeClaims(claims), time.Now(), ttl))
}

func (c *SessionCodec) ParseChallengeToken(token string) (identityaccess.MFAChallengeClaims, error) {
	verified, err := c.decode(token)
	if err != nil {
		return identityaccess.MFAChallengeClaims{}, err
	}
	claims, err := tokenprimitives.DecodeChallenge(verified, time.Now())
	return identityaccess.MFAChallengeClaims(claims), err
}

// decode verifies the signature. Expiry policy belongs to the particular
// token operation below.
func (c *SessionCodec) decode(token string) (jwt.Token, error) {
	return c.signer.Decode(token)
}

func (c *SessionCodec) ParseAccessToken(token string) (identityaccess.SessionClaims, error) {
	claims, err := c.ParseAccessTokenAllowExpired(token)
	if err != nil {
		return identityaccess.SessionClaims{}, err
	}
	if claims.ExpiresAt > 0 && claims.ExpiresAt < time.Now().Unix() {
		return identityaccess.SessionClaims{}, errors.New("access token expired")
	}
	return claims, nil
}

func (c *SessionCodec) ParseAccessTokenAllowExpired(token string) (identityaccess.SessionClaims, error) {
	verified, err := c.decode(token)
	if err != nil {
		return identityaccess.SessionClaims{}, err
	}
	claims, err := tokenprimitives.DecodeAccess(verified)
	return identityaccess.SessionClaims(claims), err
}

// ParseRefreshToken verifies signatures; persisted sessions own expiry and revocation.
func (c *SessionCodec) ParseRefreshToken(token string) (identityaccess.RefreshClaims, error) {
	verified, err := c.decode(token)
	if err != nil {
		return identityaccess.RefreshClaims{}, err
	}
	claims, err := tokenprimitives.DecodeRefresh(verified)
	return identityaccess.RefreshClaims(claims), err
}

// IssueHolidayParentSetupToken signs the setup link of a parent account at a
// Träger (ADR 0216 point 5).
func (c *SessionCodec) IssueHolidayParentSetupToken(claims identityaccess.HolidayParentSetupClaims, ttl time.Duration) (string, error) {
	return c.sign(tokenprimitives.EncodeHolidayParentSetup(tokenprimitives.HolidayParentSetupClaims(claims), time.Now(), ttl))
}

// ParseHolidayParentSetupToken accepts only an unexpired setup token.
func (c *SessionCodec) ParseHolidayParentSetupToken(token string) (identityaccess.HolidayParentSetupClaims, error) {
	verified, err := c.decode(token)
	if err != nil {
		return identityaccess.HolidayParentSetupClaims{}, err
	}
	claims, err := tokenprimitives.DecodeHolidayParentSetup(verified, time.Now())
	return identityaccess.HolidayParentSetupClaims(claims), err
}
