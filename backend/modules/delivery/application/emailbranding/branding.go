// Package emailbranding builds absolute logo URLs for transactional e-mails so
// every template renders the school logo (header) and the moto logo (footer)
// consistently, regardless of which service enqueues the mail. The logic used
// to live private to the enrollment package; it is shared here because the
// parent-announcement mail needs the identical branding.
package emailbranding

import "strings"

const (
	motoLogoPath           = "/images/moto-logo-mit-schriftzug.png"
	loginImageUploadPrefix = "/uploads/login-images/"
)

// MotoLogoURL returns the absolute URL of the moto footer logo for the given
// public base URL (e.g. PARENTS_URL). An empty base yields the bare path.
func MotoLogoURL(baseURL string) string {
	return AbsoluteURL(baseURL, motoLogoPath)
}

// SchoolLogoURL turns a school's stored login-image reference into an absolute
// URL usable in an e-mail. An uploaded image (/uploads/login-images/<file>) is
// rewritten to the public read endpoint; an already-absolute URL passes through
// unchanged; anything else is joined onto baseURL. Empty input yields "".
func SchoolLogoURL(baseURL string, rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if strings.HasPrefix(rawURL, loginImageUploadPrefix) {
		filename := strings.TrimPrefix(rawURL, loginImageUploadPrefix)
		if filename == "" || strings.Contains(filename, "/") {
			return ""
		}
		return AbsoluteURL(baseURL, "/api/public/login-image/"+filename)
	}
	return AbsoluteURL(baseURL, rawURL)
}

// AbsoluteURL joins a relative path onto baseURL. Absolute inputs (http/https)
// pass through untouched; an empty rawURL yields ""; an empty baseURL yields the
// rawURL as-is.
func AbsoluteURL(baseURL string, rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return rawURL
	}
	if strings.HasPrefix(rawURL, "/") {
		return baseURL + rawURL
	}
	return baseURL + "/" + rawURL
}
