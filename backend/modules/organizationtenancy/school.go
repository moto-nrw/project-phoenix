package organizationtenancy

import (
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
	"time"
)

type InvalidSchoolError struct{ Reason string }

func (e *InvalidSchoolError) Error() string { return e.Reason }
func (e *InvalidSchoolError) Unwrap() error { return ErrInvalidSchool }

var (
	ErrSchoolNotFound       = errors.New("school not found")
	ErrInvalidSchool        = errors.New("invalid school")
	ErrSchoolSlugConflict   = errors.New("school slug already exists in organization")
	ErrSchoolDomainConflict = errors.New("school subdomain already exists")
	ErrSchoolAlreadyDeleted = errors.New("school is already deleted")
	ErrSchoolNotDeleted     = errors.New("school is not deleted")
	ErrOrganizationDeleted  = errors.New("school organization is deleted")
)

type School struct {
	ID             int64      `json:"id"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	OrganizationID int64      `json:"organization_id"`
	Name           string     `json:"name"`
	Slug           string     `json:"slug"`
	Subdomain      string     `json:"subdomain"`
	Active         bool       `json:"active"`
	Hidden         bool       `json:"hidden"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
	Settings       string     `json:"settings,omitempty"`
	Address        string     `json:"address,omitempty"`
	City           string     `json:"city,omitempty"`
	Zip            string     `json:"zip,omitempty"`
	Phone          string     `json:"phone,omitempty"`
	Email          string     `json:"email,omitempty"`
	// ChildQuotaBundles and ChildQuotaBundleSize are the contracted
	// Kinderkontingent (#3567); no bundles means no limit.
	ChildQuotaBundles    *int          `json:"child_quota_bundles"`
	ChildQuotaBundleSize int           `json:"child_quota_bundle_size"`
	Organization         *Organization `json:"organization,omitempty"`
}

func (s School) IsDeleted() bool { return s.DeletedAt != nil }

// The keys a school's settings JSON carries its branding image under. The
// login image serves as the fallback so a school that only set that one
// still brands its pages.
const (
	schoolLogoURLKey       = "logoUrl"
	schoolLoginImageURLKey = "loginImageUrl"
)

// LogoURL is the school's branding image; malformed settings carry none.
func (s School) LogoURL() string { return SchoolLogoURL(s.Settings) }

// SchoolLogoURL reads the branding image out of a school's settings JSON:
// the logo, else the login image, trimmed. Malformed settings carry none.
func SchoolLogoURL(settingsJSON string) string {
	settingsJSON = strings.TrimSpace(settingsJSON)
	if settingsJSON == "" {
		return ""
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return ""
	}
	for _, key := range []string{schoolLogoURLKey, schoolLoginImageURLKey} {
		value, ok := settings[key].(string)
		if ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// Operational reports a school accounts can work in: switched active and
// not deleted.
func (s School) Operational() bool { return s.Active && !s.IsDeleted() }

// ChildQuota returns the school's Kinderkontingent, nil when it has none.
func (s School) ChildQuota() *ChildQuota {
	if s.ChildQuotaBundles == nil {
		return nil
	}
	return &ChildQuota{Bundles: *s.ChildQuotaBundles, BundleSize: s.ChildQuotaBundleSize}
}

type CreateSchool struct {
	OrganizationID int64
	Name           string
	Slug           string
	Subdomain      string
	Active         bool
	Hidden         bool
	Settings       string
	Address        string
	City           string
	Zip            string
	Phone          string
	Email          string
}

type UpdateSchool struct {
	ID             int64
	OrganizationID int64
	Name           string
	Slug           string
	Subdomain      string
	Active         bool
	Hidden         bool
	Settings       string
	Address        string
	City           string
	Zip            string
	Phone          string
	Email          string
}

// NormalizeCreateSchool trims the input the way CreateSchool does and
// reports the first school rule it breaks, so a caller can reject bad input
// before it reads anything.
func NormalizeCreateSchool(input *CreateSchool) error {
	if input == nil {
		return invalidSchool("school is required")
	}
	NormalizeSchool(&input.Name, &input.Slug, &input.Subdomain)
	if err := ValidateSchool(input.OrganizationID, input.Name, input.Slug, input.Subdomain, input.Email); err != nil {
		return err
	}
	return ValidateSchoolContactEmail(input.Email)
}

// ValidateSchoolContactEmail checks the contact address a school must have.
// Mail is sent from a central noreply address, so replies from parents reach
// the school only through this address, and moto must not receive them
// (#2872). It is required and must be a bare mailbox address.
func ValidateSchoolContactEmail(email string) error {
	if email == "" {
		return invalidSchool("email is required")
	}
	if len(email) > MaxEmailLength {
		return invalidSchool("email must not exceed 255 characters")
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return invalidSchool("email must be a valid address")
	}
	return nil
}

// ValidateSchool checks the shared school fields after normalization.
func ValidateSchool(organizationID int64, name, slug, subdomain, email string) error {
	if organizationID <= 0 {
		return invalidSchool("organization ID is required")
	}
	if name == "" {
		return invalidSchool("name is required")
	}
	if len(name) > MaxNameLength {
		return invalidSchool("name must not exceed 200 characters")
	}
	if err := ValidateSlug(slug); err != nil {
		return invalidSchool(err.Error())
	}
	if err := ValidateSubdomain(subdomain); err != nil {
		return err
	}
	if len(email) > MaxEmailLength {
		return invalidSchool("email must not exceed 255 characters")
	}
	return nil
}

// ValidateSubdomain checks a school subdomain against the DNS and reserved-name rules.
func ValidateSubdomain(subdomain string) error {
	if subdomain == "" {
		return invalidSchool("subdomain is required")
	}
	if len(subdomain) > MaxDNSLabelLength {
		return invalidSchool("subdomain must not exceed 63 characters (DNS label limit)")
	}
	if !slugPattern.MatchString(subdomain) {
		return invalidSchool("subdomain must be a valid DNS label: lowercase letters, numbers, and hyphens (no leading/trailing hyphens)")
	}
	if reservedSlugs[subdomain] {
		return invalidSchool("subdomain is reserved for infrastructure use")
	}
	return nil
}

// NormalizeSchool trims school names and normalizes slugs and subdomains.
func NormalizeSchool(name, slug, subdomain *string) {
	*name = strings.TrimSpace(*name)
	*slug = strings.TrimSpace(strings.ToLower(*slug))
	*subdomain = strings.TrimSpace(strings.ToLower(*subdomain))
}

func invalidSchool(reason string) error { return &InvalidSchoolError{Reason: reason} }
