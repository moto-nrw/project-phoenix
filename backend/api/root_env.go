package api

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// RootEnvironment is the process configuration every typed root builds its
// owners over (ADR 0059): the logger, the configuration snapshot, the clock,
// the validated portal links and the observers. It holds no owner value.
type RootEnvironment struct {
	Logger *slog.Logger
	Config ServiceConfig
	// Now is the clock of the services; nil uses the real one. Today is its
	// calendar day.
	Now   func() time.Time
	Today func() timezone.Date
	Links ServiceLinks
	// AllowAuditRootWrites lets the Audit trails append without a tenant
	// transaction; only test roots set it.
	AllowAuditRootWrites bool
	Observe              RootObservers
}

// ServiceLinks are the validated bases the mails and the CalDAV feed link to
// and the token lifetimes the configuration names.
type ServiceLinks struct {
	FrontendURL              string
	PublicAPIURL             string
	ParentsURL               string
	SchoolURL                string
	InvitationTokenExpiry    time.Duration
	PasswordResetTokenExpiry time.Duration
}

// RootObservers record the operations of the owners the roots compose. The
// serving roots hand them the metrics sinks; test roots pass no-ops.
type RootObservers struct {
	Communication   func(communicationCompose.Observation)
	AuditAppend     AuditAppendObserver
	Delivery        DeliveryObserver
	DurableDelivery DurableDeliveryObserver
	DeviceFleet     DeviceFleetObserver
	IdentityAccess  IdentityAccessObserver
	FileStorage     func(filestorageCompose.Observation)
	SettingsLookup  settingsCompose.SettingsLookupObserver
	// SettingsSideEffectFailure records a failed settings side effect.
	SettingsSideEffectFailure settingsCompose.SideEffectFailureObserver
	// CarePlan records the excused-absence workflow as Care Plan operations.
	CarePlan func(carePlanCompose.Observation)
}

// newRootEnvironment validates the portal links of the configuration and
// fixes the clock: production requires https links.
func newRootEnvironment(cfg ServiceConfig, logger *slog.Logger, clock func() time.Time, observe RootObservers) (RootEnvironment, error) {
	if logger == nil {
		return RootEnvironment{}, fmt.Errorf("root environment: logger is required")
	}
	links, err := resolveServiceLinks(cfg)
	if err != nil {
		return RootEnvironment{}, err
	}
	return RootEnvironment{
		Logger: logger, Config: cfg, Now: clock, Today: timezone.CalendarDateClock(clock),
		Links: links, Observe: observe,
	}, nil
}

// resolveServiceLinks validates the staff, API, parents-portal and
// school-portal bases the mails and the CalDAV feed link to: production
// requires https.
func resolveServiceLinks(cfg ServiceConfig) (ServiceLinks, error) {
	production := strings.ToLower(cfg.AppEnv) == "production"
	links := ServiceLinks{
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
	} {
		value, err := requiredLink(link.name, link.raw, production)
		if err != nil {
			return ServiceLinks{}, err
		}
		*link.target = value
	}
	return links, nil
}
