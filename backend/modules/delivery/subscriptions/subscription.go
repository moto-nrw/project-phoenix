package subscriptions

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Push subscription portal constants: through which portal the device
// registered. Tenant/admin-scoped pushes go only to staff devices, guardian
// pushes only to parent devices.
const (
	PushPortalStaff  = "staff"
	PushPortalParent = "parent"
	// PushPortalSchool marks a device registered through "moto schule" (#2208).
	PushPortalSchool = "school"
)

var trustedPushServiceHosts = map[string]struct{}{
	"fcm.googleapis.com":                {},
	"updates.push.services.mozilla.com": {},
	"web.push.apple.com":                {},
}

// ValidatePushEndpoint restricts outbound delivery to browser-managed push
// services. Subscription endpoints are otherwise attacker-controlled URLs and
// must never turn the notification worker into a general-purpose HTTP client.
func ValidatePushEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint must be a valid URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Hostname() == "" {
		return errors.New("endpoint must be an https URL")
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return errors.New("endpoint must not contain user information or a fragment")
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return errors.New("endpoint must use the default https port")
	}

	host := strings.ToLower(parsed.Hostname())
	if _, ok := trustedPushServiceHosts[host]; ok {
		return nil
	}
	if strings.HasSuffix(host, ".notify.windows.com") &&
		host != "notify.windows.com" {
		return nil
	}
	return errors.New("endpoint host is not a trusted push service")
}

// PushSubscription is one browser/device registration for Web Push
// (RFC 8030). endpoint/p256dh/auth come verbatim from the browser's
// PushSubscription JSON. One account can hold many rows (multiple devices);
// a guardian linked to several schools holds one row per school for the
// same endpoint.
type PushSubscription struct {
	ID            int64     `json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	TenantID      int64     `json:"tenant_id"`
	AccountID     int64     `json:"account_id"`
	Portal        string    `json:"portal"`
	Endpoint      string    `json:"endpoint"`
	P256dh        string    `json:"-"`
	Auth          string    `json:"-"`
	UserAgent     string    `json:"user_agent"`
	TokenFamilyID string    `json:"-"`
}

// Validate ensures push subscription data is valid.
func (s *PushSubscription) Validate() error {
	if s.AccountID <= 0 {
		return errors.New("account_id is required")
	}
	if s.Portal != PushPortalStaff && s.Portal != PushPortalParent && s.Portal != PushPortalSchool {
		return errors.New("portal must be 'staff', 'parent' or 'school'")
	}
	if strings.TrimSpace(s.Endpoint) == "" {
		return errors.New("endpoint is required")
	}
	if err := ValidatePushEndpoint(s.Endpoint); err != nil {
		return err
	}
	if strings.TrimSpace(s.P256dh) == "" || strings.TrimSpace(s.Auth) == "" {
		return errors.New("subscription keys are required")
	}
	return nil
}
