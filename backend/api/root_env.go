package api

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// newRootEnvironment validates the portal links of the configuration and
// fixes the clock: production requires https links.
func newRootEnvironment(cfg owners.ServiceConfig, logger *slog.Logger, clock func() time.Time, observe owners.RootObservers) (owners.RootEnvironment, error) {
	if logger == nil {
		return owners.RootEnvironment{}, fmt.Errorf("root environment: logger is required")
	}
	links, err := resolveServiceLinks(cfg)
	if err != nil {
		return owners.RootEnvironment{}, err
	}
	return owners.RootEnvironment{
		Logger: logger, Config: cfg, Now: clock, Today: timezone.CalendarDateClock(clock),
		Links: links, Observe: observe,
	}, nil
}

// resolveServiceLinks validates the staff, API, parents-portal,
// school-portal and caterer-portal bases the mails and the CalDAV feed link to: production
// requires https.
func resolveServiceLinks(cfg owners.ServiceConfig) (owners.ServiceLinks, error) {
	production := strings.ToLower(cfg.AppEnv) == "production"
	links := owners.ServiceLinks{
		InvitationTokenExpiry:    identityaccess.InvitationTokenExpiry(cfg.InvitationTokenExpiryHours),
		PasswordResetTokenExpiry: identityaccess.PasswordResetTokenExpiry(cfg.PasswordResetExpiryMinutes),
	}
	for _, link := range []struct {
		name   string
		raw    string
		target *string
	}{
		{"FRONTEND_URL", cfg.FrontendURL, &links.FrontendURL},
		{"NEXT_PUBLIC_API_URL", cfg.PublicAPIURL, &links.PublicAPIURL},
		// Every parent-facing email link (status, decision emails,
		// guardian invitation accept).
		{"PARENTS_URL", cfg.ParentsURL, &links.ParentsURL},
		// Lehrkraft invitation links land on the school portal, where the
		// accept flow lives (#2207).
		{"SCHOOL_URL", cfg.SchoolURL, &links.SchoolURL},
		// The caterer's invitation link lands on the caterer portal
		// (ADR 0075).
		{"CATERER_URL", cfg.CatererURL, &links.CatererURL},
	} {
		value, err := requiredLink(link.name, link.raw, production)
		if err != nil {
			return owners.ServiceLinks{}, err
		}
		*link.target = value
	}
	return links, nil
}
