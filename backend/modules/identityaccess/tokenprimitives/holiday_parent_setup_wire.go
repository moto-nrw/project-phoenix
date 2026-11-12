package tokenprimitives

import (
	"errors"
	"fmt"
	"time"
)

// The setup link of a parent account at a Träger (ADR 0216 point 5) is a
// signed token with the address, the Träger and an expiry. It is stored
// nowhere and is no session: it names no account and carries its own
// purpose flag, which every session and MFA decoder refuses, and its decoder
// refuses every claim that belongs to those.

const holidayParentSetupFlag = "holiday_parent_setup"

// HolidayParentSetupClaims is the content of a setup token: the normalized
// address and the Träger of the holiday registration it was mailed for.
type HolidayParentSetupClaims struct {
	Email          string
	OrganizationID int64
	ExpiresAt      int64
}

// HolidayParentSetupWire is the signed form of a setup token.
type HolidayParentSetupWire struct {
	Purpose        bool   `json:"holiday_parent_setup"`
	Email          string `json:"email"`
	OrganizationID int64  `json:"org_id"`
	IssuedAt       int64  `json:"iat"`
	ExpiresAt      int64  `json:"exp"`
}

// EncodeHolidayParentSetup is the wire form of a setup token.
func EncodeHolidayParentSetup(c HolidayParentSetupClaims, issuedAt time.Time, ttl time.Duration) HolidayParentSetupWire {
	return HolidayParentSetupWire{
		Purpose: true, Email: c.Email, OrganizationID: c.OrganizationID,
		IssuedAt: issuedAt.Unix(), ExpiresAt: issuedAt.Add(ttl).Unix(),
	}
}

// DecodeHolidayParentSetup reads a verified setup token that has not
// expired.
func DecodeHolidayParentSetup(token Token, now time.Time) (HolidayParentSetupClaims, error) {
	wire, err := wireClaims(token)
	if err != nil {
		return HolidayParentSetupClaims{}, err
	}
	return decodeHolidayParentSetup(wire, now)
}

// sessionClaimsOfOtherTokens are claims only a session, refresh or MFA token
// carries; a setup token with one of them is refused.
var sessionClaimsOfOtherTokens = []string{"id", "sub", "token", "account_id", "scope", "family_id", pendingChallengeFlag, pendingEnrollmentFlag}

func decodeHolidayParentSetup(wire map[string]any, now time.Time) (HolidayParentSetupClaims, error) {
	if !flag(wire, holidayParentSetupFlag) {
		return HolidayParentSetupClaims{}, errors.New("token is not a holiday parent setup token")
	}
	for _, claim := range sessionClaimsOfOtherTokens {
		if _, present := wire[claim]; present {
			return HolidayParentSetupClaims{}, fmt.Errorf("holiday parent setup token carries the foreign claim %s", claim)
		}
	}
	claims := HolidayParentSetupClaims{Email: text(wire, "email"), OrganizationID: number(wire, "org_id"), ExpiresAt: number(wire, "exp")}
	switch {
	case claims.Email == "":
		return HolidayParentSetupClaims{}, errors.New("missing required claim: email")
	case claims.OrganizationID <= 0:
		return HolidayParentSetupClaims{}, errors.New("missing required claim: org_id")
	case claims.ExpiresAt <= 0 || claims.ExpiresAt < now.Unix():
		return HolidayParentSetupClaims{}, errors.New("holiday parent setup token expired")
	}
	return claims, nil
}
