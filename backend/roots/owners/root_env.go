package owners

import (
	"log/slog"
	"time"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
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
	Now     func() time.Time
	Today   func() timezone.Date
	Links   ServiceLinks
	Observe RootObservers
}

// ServiceLinks are the validated bases the mails and the CalDAV feed link to
// and the token lifetimes the configuration names.
type ServiceLinks struct {
	FrontendURL  string
	PublicAPIURL string
	ParentsURL   string
	SchoolURL    string
	CatererURL   string
	// TraegerURL is the Träger-Portal base invitations link to (ADR 0084).
	TraegerURL               string
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
