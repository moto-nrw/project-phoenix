package domain

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// The passkey ceremony rules (#3331): which origin may start a ceremony,
// which relying-party ID it runs under, and how a stored credential is
// summarised. They are the same for both portals; only the host a ceremony
// is pinned to differs.

var (
	// ErrPasskeyOriginInvalid refuses a ceremony started from a host that is
	// not the portal's own.
	ErrPasskeyOriginInvalid = errors.New("passkey origin is invalid")
	// ErrPasskeySessionInvalid refuses a ceremony whose stored state does
	// not match the request completing it.
	ErrPasskeySessionInvalid = errors.New("passkey session is invalid")
	// ErrPasskeyNotFound reports a revocation that matched no active
	// credential of that account or operator.
	ErrPasskeyNotFound = errors.New("passkey not found")
)

const (
	// PasskeyUserHandleBytes is the size of a freshly minted WebAuthn user
	// handle.
	PasskeyUserHandleBytes = 32
	// PasskeyCeremonyTimeout bounds how long a started ceremony may take.
	PasskeyCeremonyTimeout = 10 * time.Minute
	// maxPasskeyNameLength bounds the label a user gives a credential.
	maxPasskeyNameLength = 80
	// DefaultPasskeyName is the label a registration without one gets.
	DefaultPasskeyName = "Passkey"
)

// PasskeyCredentialSummary is what the portals list: the credential's
// identity as a string, its label and its use.
type PasskeyCredentialSummary struct {
	ID         string
	Name       string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// SummarizeAccountPasskey renders one stored account credential.
func SummarizeAccountPasskey(credential AccountPasskeyCredential) PasskeyCredentialSummary {
	return PasskeyCredentialSummary{
		ID: strconv.FormatInt(credential.ID, 10), Name: credential.Name,
		CreatedAt: credential.CreatedAt, LastUsedAt: credential.LastUsedAt,
	}
}

// SummarizeOperatorPasskey renders one stored operator credential.
func SummarizeOperatorPasskey(credential OperatorPasskeyCredential) PasskeyCredentialSummary {
	return PasskeyCredentialSummary{
		ID: strconv.FormatInt(credential.ID, 10), Name: credential.Name,
		CreatedAt: credential.CreatedAt, LastUsedAt: credential.LastUsedAt,
	}
}

// NormalizePasskeyName trims the label and caps its length.
func NormalizePasskeyName(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > maxPasskeyNameLength {
		value = value[:maxPasskeyNameLength]
	}
	return value
}

// GeneratePasskeyUserHandle mints a fresh WebAuthn user handle for an
// identity that has none yet.
func GeneratePasskeyUserHandle() ([]byte, error) {
	handle := make([]byte, PasskeyUserHandleBytes)
	if _, err := rand.Read(handle); err != nil {
		return nil, fmt.Errorf("read random bytes for passkey user handle: %w", err)
	}
	return handle, nil
}

// HostWithoutPort reduces a URL or host value to its lower-case hostname.
func HostWithoutPort(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.Contains(value, "://") {
		if parsed, err := url.Parse(value); err == nil {
			value = parsed.Host
		}
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		return strings.ToLower(host)
	}
	return strings.TrimSuffix(value, ".")
}

// OriginHostWithoutPort validates a browser origin and returns its host.
func OriginHostWithoutPort(origin string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("origin must include scheme and host")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", errors.New("unsupported origin scheme")
	}
	return HostWithoutPort(parsed.Host), nil
}

// ResolvePasskeyRPIDForOrigin returns the relying-party ID a ceremony runs
// under. A configured localhost relying party follows the request's own
// host, so every school subdomain works in development.
func ResolvePasskeyRPIDForOrigin(configuredRPID, origin string) (string, error) {
	rpID := HostWithoutPort(configuredRPID)
	if rpID != "localhost" {
		return rpID, nil
	}
	return OriginHostWithoutPort(origin)
}

// ValidateTenantPasskeyOrigin refuses a ceremony that did not start on the
// school's own subdomain. In development both `localhost` and
// `{subdomain}.localhost` are accepted.
func ValidateTenantPasskeyOrigin(origin, subdomain, tenantDomain string) error {
	originHost, err := OriginHostWithoutPort(origin)
	if err != nil {
		return ErrPasskeyOriginInvalid
	}
	root := strings.ToLower(HostWithoutPort(tenantDomain))
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	if root == "localhost" {
		if originHost == "localhost" || originHost == subdomain+".localhost" {
			return nil
		}
		return ErrPasskeyOriginInvalid
	}
	if subdomain == "" {
		return ErrPasskeyOriginInvalid
	}
	if originHost != subdomain+"."+root {
		return ErrPasskeyOriginInvalid
	}
	return nil
}

// ValidateOperatorPasskeyOrigin refuses a ceremony that did not start on the
// operator portal's own host.
func ValidateOperatorPasskeyOrigin(origin, operatorHost string) error {
	host, err := OriginHostWithoutPort(origin)
	if err != nil {
		return ErrPasskeyOriginInvalid
	}
	if host != operatorHost {
		return ErrPasskeyOriginInvalid
	}
	return nil
}
