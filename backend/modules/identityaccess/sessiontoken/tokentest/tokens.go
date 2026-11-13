//go:build testhooks

// Package tokentest mints and reads session JWTs for tests through the
// production wire codec (tokenprimitives). It is a leaf so these helpers do
// not change how sessiontoken, and the packages importing it, compile under
// testhooks (#5990).
package tokentest

import (
	"encoding/json"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/tokenprimitives"
)

// MFA challenge scopes, the portal names of the portals with a challenge.
const (
	MFAChallengeScopeTenant   = tokenprimitives.PortalTenant
	MFAChallengeScopePlatform = tokenprimitives.PortalPlatform
	MFAChallengeScopeSchool   = tokenprimitives.PortalSchool
	MFAChallengeScopeOrg      = tokenprimitives.PortalOrg
	MFAChallengeScopeCaterer  = tokenprimitives.PortalCaterer
)

// RefreshClaims is the content of a test refresh token. A zero ExpiresAt
// gets the signer's refresh lifetime.
type RefreshClaims struct {
	ID        int
	Token     string
	TenantID  int64
	OrgID     int64
	Scope     string
	FamilyID  string
	LineageID string
	sessiontoken.CommonClaims
}

// MFAChallengeClaims is the content of a test MFA challenge token.
type MFAChallengeClaims struct {
	AccountID            int64
	Scope                string
	TenantID             int64
	ChallengeID          int64
	CredentialGeneration int64
	MFAPending           bool
	sessiontoken.CommonClaims
}

// GenTokenPair returns both an access token and a refresh token.
func GenTokenPair(a *sessiontoken.TokenAuth, accessClaims sessiontoken.AppClaims, refreshClaims RefreshClaims) (string, string, error) {
	access, err := CreateJWT(a, accessClaims)
	if err != nil {
		return "", "", err
	}
	refresh, err := CreateRefreshJWT(a, refreshClaims)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

// CreateJWT returns an access token for claims that expires after the
// signer's access lifetime.
func CreateJWT(a *sessiontoken.TokenAuth, c sessiontoken.AppClaims) (string, error) {
	now := time.Now()
	c.IssuedAt = now.Unix()
	c.ExpiresAt = now.Add(a.JwtExpiry).Unix()
	return sign(a, accessWire(c))
}

// AccessClaimMap is the access-token claim map of c with c's own iat and exp,
// for tests that alter a claim before signing.
func AccessClaimMap(c sessiontoken.AppClaims) map[string]any {
	return claimMap(accessWire(c))
}

func accessWire(c sessiontoken.AppClaims) tokenprimitives.AccessWire {
	return tokenprimitives.EncodeAccess(tokenprimitives.AccessClaims{
		AccountID: int64(c.ID), Email: c.Sub, Username: c.Username, FirstName: c.FirstName, LastName: c.LastName,
		Roles: c.Roles, Permissions: c.Permissions, IsAdmin: c.IsAdmin, Scope: c.Scope,
		TenantID: c.TenantID, OrgID: c.OrgID, FamilyID: c.FamilyID,
		ReadOnly: c.ReadOnly, ActingAdminID: c.ActingAdminID, PreviewID: c.PreviewID, ViaTraeger: c.ViaTraeger,
	}, time.Unix(c.IssuedAt, 0), time.Unix(c.ExpiresAt, 0))
}

// CreateRefreshJWT returns a refresh token for c.
func CreateRefreshJWT(a *sessiontoken.TokenAuth, c RefreshClaims) (string, error) {
	now := time.Now()
	return sign(a, tokenprimitives.EncodeRefresh(tokenprimitives.RefreshClaims{
		AccountID: int64(c.ID), Token: c.Token, TenantID: c.TenantID, OrgID: c.OrgID, Scope: c.Scope,
		FamilyID: c.FamilyID, LineageID: c.LineageID, ExpiresAt: c.ExpiresAt,
	}, now, now.Add(a.JwtRefreshExpiry)))
}

// ParseExpiredAccessJWT reads the claims of a signed access token without
// the expiry check, for tests that read a token that has already expired.
func ParseExpiredAccessJWT(a *sessiontoken.TokenAuth, tokenString string) (*sessiontoken.AppClaims, error) {
	token, err := a.JwtAuth.Decode(tokenString)
	if err != nil {
		return nil, err
	}
	claims, err := tokenprimitives.DecodeAccess(token)
	if err != nil {
		return nil, err
	}
	return &sessiontoken.AppClaims{
		ID: int(claims.AccountID), Sub: claims.Email, Username: claims.Username, FirstName: claims.FirstName, LastName: claims.LastName,
		Roles: claims.Roles, Permissions: claims.Permissions, IsAdmin: claims.IsAdmin, Scope: claims.Scope,
		TenantID: claims.TenantID, OrgID: claims.OrgID, FamilyID: claims.FamilyID,
		ReadOnly: claims.ReadOnly, ActingAdminID: claims.ActingAdminID, PreviewID: claims.PreviewID, ViaTraeger: claims.ViaTraeger,
		CommonClaims: sessiontoken.CommonClaims{ExpiresAt: claims.ExpiresAt},
	}, nil
}

// CreateMFAChallengeJWT mints an MFA challenge token with the given TTL.
func CreateMFAChallengeJWT(a *sessiontoken.TokenAuth, c MFAChallengeClaims, ttl time.Duration) (string, error) {
	return sign(a, tokenprimitives.EncodeChallenge(tokenprimitives.ChallengeClaims{
		AccountID: c.AccountID, Scope: c.Scope, TenantID: c.TenantID, ChallengeID: c.ChallengeID,
		CredentialGeneration: c.CredentialGeneration,
	}, time.Now(), ttl))
}

// ParseMFAChallengeJWT reads an MFA challenge token the way the Identity &
// Access codec does, expiry included.
func ParseMFAChallengeJWT(a *sessiontoken.TokenAuth, tokenString string) (*MFAChallengeClaims, error) {
	token, err := a.JwtAuth.Decode(tokenString)
	if err != nil {
		return nil, err
	}
	claims, err := tokenprimitives.DecodeChallenge(token, time.Now())
	if err != nil {
		return nil, err
	}
	return &MFAChallengeClaims{
		AccountID: claims.AccountID, Scope: claims.Scope, TenantID: claims.TenantID, ChallengeID: claims.ChallengeID,
		CredentialGeneration: claims.CredentialGeneration, MFAPending: true,
	}, nil
}

// CreateMFAEnrollmentJWT mints an MFA enrollment token with the given TTL.
func CreateMFAEnrollmentJWT(a *sessiontoken.TokenAuth, c sessiontoken.MFAEnrollmentClaims, ttl time.Duration) (string, error) {
	wire, err := tokenprimitives.EncodeEnrollment(tokenprimitives.EnrollmentClaims{
		AccountID: c.AccountID, Scope: c.Scope, TenantID: c.TenantID, CredentialGeneration: c.CredentialGeneration,
	}, time.Now(), ttl)
	if err != nil {
		return "", err
	}
	return sign(a, wire)
}

// ParseMFAEnrollmentJWT reads an MFA enrollment token, expiry included.
func ParseMFAEnrollmentJWT(a *sessiontoken.TokenAuth, tokenString string) (*sessiontoken.MFAEnrollmentClaims, error) {
	token, err := a.JwtAuth.Decode(tokenString)
	if err != nil {
		return nil, err
	}
	verified := tokenprimitives.DecodeVerified(token, time.Now())
	if verified.EnrollmentErr != nil {
		return nil, verified.EnrollmentErr
	}
	claims := verified.Enrollment
	return &sessiontoken.MFAEnrollmentClaims{
		AccountID: claims.AccountID, Scope: claims.Scope, TenantID: claims.TenantID, CredentialGeneration: claims.CredentialGeneration, MFAEnrollmentPending: true,
		CommonClaims: sessiontoken.CommonClaims{ExpiresAt: claims.ExpiresAt},
	}, nil
}

func sign(a *sessiontoken.TokenAuth, wire any) (string, error) {
	_, tokenString, err := a.JwtAuth.Encode(claimMap(wire))
	return tokenString, err
}

// claimMap is the JSON object of a wire value.
func claimMap(wire any) map[string]any {
	encoded, err := json.Marshal(wire)
	if err != nil {
		panic(err)
	}
	var claims map[string]any
	if err := json.Unmarshal(encoded, &claims); err != nil {
		panic(err)
	}
	return claims
}
