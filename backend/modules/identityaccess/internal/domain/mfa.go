package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Multi-factor authentication (#3331): the second factor of an account or
// operator login. The decisions that need no database live here — the
// lockout window, the admin-override verdict, the code and the
// trusted-device cookie — so the flows above them read as one story and the
// clock and the random source stay injected.

// The portal a challenge belongs to. A code is only redeemable at the
// surface it was started for.
const (
	MFAChallengeScopeTenant   = "tenant"
	MFAChallengeScopePlatform = "platform"
	MFAChallengeScopeSchool   = "school"
)

// MFA policy constants. The account values are the fallbacks the school's
// security.* settings override; the operator values are fixed, because every
// moto operator faces the same posture.
const (
	// MFAChallengeTTL is the e-mail code lifetime and the challenge JWT TTL.
	MFAChallengeTTL = 10 * time.Minute
	// MFALockoutThreshold is the number of failed verifies before the
	// cooldown starts.
	MFALockoutThreshold = 5
	// MFALockoutDuration matches the PIN lockout.
	MFALockoutDuration = 15 * time.Minute
	// MFAEmailRateLimitWindow is the sliding window the issued-code cap
	// counts over.
	MFAEmailRateLimitWindow = 15 * time.Minute
	// MFAEmailRateLimitMaxSent is the hard cap of codes per account and
	// window. It is an abuse defense, not a UX knob, and stays in code.
	MFAEmailRateLimitMaxSent = 3
	// MFATrustedDeviceCookieDefaultDays is the remember-device lifetime the
	// school's security.mfa_trusted_device_days setting overrides.
	MFATrustedDeviceCookieDefaultDays = 90
	// MFAEmailCodeLength is the number of decimal digits in an e-mail code.
	MFAEmailCodeLength = 6
	// MFATrustedDeviceTokenBytes is the random-source size of a
	// trusted-device token: 256 bits, far above the OWASP recommendation.
	MFATrustedDeviceTokenBytes = 32
	// MFAAuditFallbackIP is the sentinel written to the INET NOT NULL audit
	// column when an event has no client address (lockout rollover, admin
	// disable from the operator surface).
	MFAAuditFallbackIP = "0.0.0.0"

	// OperatorMFAChallengeTTL is the operator e-mail code lifetime.
	OperatorMFAChallengeTTL = 10 * time.Minute
	// OperatorMFARateLimitWindow and OperatorMFARateLimitMaxSent cap the
	// codes one operator may have sent.
	OperatorMFARateLimitWindow  = 15 * time.Minute
	OperatorMFARateLimitMaxSent = 3
	// OperatorMFATrustedDeviceDuration is the operator remember-device
	// lifetime; operators have no per-school setting.
	OperatorMFATrustedDeviceDuration = 90 * 24 * time.Hour
)

// trustedDeviceHKDFInfo derives a per-purpose secret from the JWT secret so
// the remember-device HMAC key and the JWT signing key stay independent.
const trustedDeviceHKDFInfo = "mfa-trusted-device-v1"

// MFA errors. The messages are generic on purpose; the detail stays in the
// logs.
var (
	ErrMFAChallengeTokenInvalid = errors.New("invalid or expired challenge token")
	ErrMFACodeInvalid           = errors.New("invalid or expired code")
	ErrMFALocked                = errors.New("account locked due to too many failed attempts")
	ErrMFARateLimited           = errors.New("too many code requests, please wait")
	ErrMFANotEnrolled           = errors.New("mfa not enrolled for this account")
	ErrMFAAlreadyEnrolled       = errors.New("mfa already enrolled for this account")
	ErrMFAPermissionDenied      = errors.New("permission denied")
	ErrMFAInvalidOverride       = errors.New("invalid mfa override value")
	ErrMFAUnsupportedScope      = errors.New("operator-scope MFA is wired up in a separate phase")
	// ErrMFAReasonRequired reports an admin override without a reason.
	ErrMFAReasonRequired = errors.New("reason is required for admin override")
	// ErrMFAGlobalReasonRequired reports a platform-wide override without a
	// reason.
	ErrMFAGlobalReasonRequired = errors.New("reason is required for global mfa override")
	// ErrMFATargetTenantRequired reports a school-scoped override write
	// without the school it belongs to.
	ErrMFATargetTenantRequired = errors.New("target tenant id is required for set_override")
)

// MFA admin-override values.
const (
	MFAAdminOverrideNone     = "none"
	MFAAdminOverrideForceOff = "force_off"
	MFAAdminOverrideForceOn  = "force_on"
)

// MFA modes a school's security.mfa_mode setting takes.
const (
	MFAModeOff            = "off"
	MFAModeRequiredAll    = "required_all"
	MFAModeRequiredAdmins = "required_admins"
)

// MFAOverrideSetByTypeOperator marks an override an operator wrote.
const MFAOverrideSetByTypeOperator = "operator"

// MFAMethodEmail is the only enrolled method today.
const MFAMethodEmail = "email"

// IsValidMFAAdminOverride is the allow-list every override write passes
// before the database CHECK constraint sees it.
func IsValidMFAAdminOverride(value string) bool {
	switch value {
	case MFAAdminOverrideNone, MFAAdminOverrideForceOff, MFAAdminOverrideForceOn:
		return true
	}
	return false
}

// MFAPolicy is the MFA verdict for one account at one school with the role
// predicate left unapplied: every database read (the admin overrides, the
// school's security.mfa_mode) is settled, and what remains is a pure
// function of a role set.
//
// The split exists because the school-portal mint guard settles the gate
// under the account lock it already holds: it resolves the policy freshly in
// its own transaction and applies it to the role set it read under that
// lock. Both halves can change while a login is in flight, and neither may
// slip a session through without a second factor.
//
// The zero value requires nothing.
type MFAPolicy struct {
	// override, when set, is the admin override's verdict and wins outright.
	override *bool
	mode     string
}

// MFAPolicyForced is the policy an admin override produces.
func MFAPolicyForced(required bool) MFAPolicy { return MFAPolicy{override: &required} }

// MFAPolicyForMode is the policy a school's security.mfa_mode produces.
func MFAPolicyForMode(mode string) MFAPolicy { return MFAPolicy{mode: mode} }

// RequiredFor applies the resolved policy to a concrete role-name set. Pure:
// safe to call from inside a transaction holding locks.
func (p MFAPolicy) RequiredFor(roleNames []string) bool {
	if p.override != nil {
		return *p.override
	}
	switch p.mode {
	case MFAModeRequiredAll:
		return true
	case MFAModeRequiredAdmins:
		for _, name := range roleNames {
			// A school names its own roles, so the match is
			// case-insensitive: "Admin" is the admin role too, and a
			// case-sensitive compare would let it log in without a second
			// factor.
			if strings.EqualFold(name, AdminRoleName) {
				return true
			}
		}
		return false
	default:
		// Off, unset, or an unknown value the resolution already warned
		// about.
		return false
	}
}

// KnownMFAMode reports whether mode is one the policy understands. An
// unknown value is treated as off and logged by the caller.
func KnownMFAMode(mode string) bool {
	switch mode {
	case MFAModeOff, MFAModeRequiredAll, MFAModeRequiredAdmins:
		return true
	}
	return false
}

// OverrideVerdict turns a stored override value into a forced policy. The
// second result is false for "none" and for anything unrecognised, which
// leaves the school's mode to decide.
func OverrideVerdict(override string) (MFAPolicy, bool) {
	switch override {
	case MFAAdminOverrideForceOff:
		return MFAPolicyForced(false), true
	case MFAAdminOverrideForceOn:
		return MFAPolicyForced(true), true
	}
	return MFAPolicy{}, false
}

// MFALocked reports whether a lockout stamp is still in the future. The
// clock is the caller's; the row only holds the fact.
func MFALocked(lockedUntil *time.Time, now time.Time) bool {
	return lockedUntil != nil && now.Before(*lockedUntil)
}

// GenerateEmailCode returns a fresh zero-padded six-digit code from
// crypto/rand.
func GenerateEmailCode() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("read random bytes for mfa email code: %w", err)
	}
	// The modulo bias for 10^6 over 2^32 is about 10^-3, negligible given
	// the ten-minute TTL, the single use and the five-attempt lockout.
	n := binary.BigEndian.Uint32(buf[:]) % 1_000_000
	return fmt.Sprintf("%06d", n), nil
}

// GenerateTrustedDeviceToken returns a fresh URL-safe random token. The
// stored hash is computed separately by HashTrustedDeviceToken.
func GenerateTrustedDeviceToken() (string, error) {
	buf := make([]byte, MFATrustedDeviceTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random bytes for trusted-device token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashTrustedDeviceToken returns the hex-encoded SHA-256 of the raw token.
// SHA-256 is enough because the token is 256 bits of randomness: there is no
// offline-guessing surface.
func HashTrustedDeviceToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

// SignTrustedDeviceToken returns rawToken + "." + base64(HMAC-SHA256). The
// cookie ships the signed form, so guessing the token bytes is not enough
// without the server secret.
func SignTrustedDeviceToken(rawToken string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(rawToken))
	return rawToken + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// VerifyTrustedDeviceToken returns the raw token of a signed cookie value
// when its signature verifies, and ("", false) for anything malformed or
// tampered with.
func VerifyTrustedDeviceToken(signedToken string, secret []byte) (string, bool) {
	dot := strings.LastIndexByte(signedToken, '.')
	if dot <= 0 || dot == len(signedToken)-1 {
		return "", false
	}
	raw := signedToken[:dot]
	signature, err := base64.RawURLEncoding.DecodeString(signedToken[dot+1:])
	if err != nil {
		return "", false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(raw))
	if subtle.ConstantTimeCompare(signature, mac.Sum(nil)) != 1 {
		return "", false
	}
	return raw, true
}

// DeriveMFASecret produces the trusted-device HMAC key from the JWT secret
// with a fixed info string, so the two keys stay cryptographically separate
// without a second environment variable.
func DeriveMFASecret(jwtSecret string) []byte {
	if jwtSecret == "" {
		return nil
	}
	mac := hmac.New(sha256.New, []byte(jwtSecret))
	mac.Write([]byte(trustedDeviceHKDFInfo))
	return mac.Sum(nil)
}

// ShortenUserAgent collapses a User-Agent string to a "Browser auf OS"
// label. It mirrors the frontend helper so the notification mail and the
// trusted-device list name one device the same way.
func ShortenUserAgent(userAgent string) string {
	if strings.TrimSpace(userAgent) == "" {
		return "Unbekanntes Gerät"
	}
	lower := strings.ToLower(userAgent)
	browser := "Browser"
	switch {
	case strings.Contains(lower, "edg/"):
		browser = "Edge"
	case strings.Contains(lower, "firefox"):
		browser = "Firefox"
	case strings.Contains(lower, "chrome") && !strings.Contains(lower, "edg/"):
		browser = "Chrome"
	case strings.Contains(lower, "safari") && !strings.Contains(lower, "chrome"):
		browser = "Safari"
	}
	osName := ""
	switch {
	case strings.Contains(lower, "iphone"):
		osName = "iPhone"
	case strings.Contains(lower, "ipad"):
		osName = "iPad"
	case strings.Contains(lower, "android"):
		osName = "Android"
	case strings.Contains(lower, "mac os"):
		osName = "macOS"
	case strings.Contains(lower, "windows"):
		osName = "Windows"
	case strings.Contains(lower, "linux"):
		osName = "Linux"
	}
	if osName == "" {
		return browser
	}
	return browser + " auf " + osName
}

// HasPermissionMatch reports whether a granted permission set contains the
// required permission, honouring the "area:*" and "admin:*" wildcards.
func HasPermissionMatch(granted []string, required string) bool {
	for _, permission := range granted {
		if permission == required || wildcardMatches(permission, required) {
			return true
		}
	}
	return false
}

func wildcardMatches(granted, required string) bool {
	if !strings.HasSuffix(granted, ":*") {
		return false
	}
	prefix := strings.TrimSuffix(granted, ":*") + ":"
	return strings.HasPrefix(required, prefix) || granted == "admin:*"
}

// MFAEvidence carries the metadata of one mfa_* authentication event. Each
// field belongs to one event shape; the recorder writes only the keys the
// shape set.
type MFAEvidence struct {
	// ChallengeID names the code an mfa_email_sent event issued.
	ChallengeID int64
	// DeviceID names the row an mfa_trusted_device_added event created.
	DeviceID int64
	// LockedUntil is the cooldown an mfa_locked event started.
	LockedUntil *time.Time
	// Admin is set on an mfa_admin_override event.
	Admin *MFAAdminEvidence
}

// MFAAdminEvidence describes one admin override attempt, accepted or
// refused. Both outcomes are recorded so scanning is forensically visible.
type MFAAdminEvidence struct {
	ActorType      string
	ActorAccountID int64
	Action         string
	// Written reports a write that went through: it adds the scope and the
	// value the override replaced. A refused attempt records the requested
	// value alone, and only when it carried one.
	Written          bool
	Scope            string
	Override         string
	PreviousOverride string
	Reason           string
}

// OperatorMFAEvidence carries the metadata of one operator MFA action.
type OperatorMFAEvidence struct {
	// Reason names why a verification failed.
	Reason string
	// LockedUntil is the cooldown an mfa_locked entry started.
	LockedUntil *time.Time
	// Override is set on the operator's account-wide MFA override.
	Override *OperatorMFAOverrideEvidence
}

// OperatorMFAOverrideEvidence records one platform-wide override
// transition.
type OperatorMFAOverrideEvidence struct {
	Action           string
	Scope            string
	Override         string
	PreviousOverride string
	Reason           string
}
