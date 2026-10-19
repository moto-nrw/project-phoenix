package care

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PortalStudent is a workflow-owned complete hydrated student snapshot, not
// an ORM row. Departure validation and normalization stay behind owner policy.
type PortalStudent struct {
	ID                       int64
	TenantID                 int64
	CreatedAt                time.Time
	UpdatedAt                time.Time
	PersonID                 int64
	SchoolClass              string
	GroupID                  *int64
	AddressStreet            *string
	AddressCity              *string
	AddressPostalCode        *string
	ExtraInfo                *string
	SupervisorNotes          *string
	HealthInfo               *string
	PickupStatus             *string
	DepartureDays            PortalDepartureDays
	AllowedDepartureModes    PortalAllowedDepartureModes
	DepartureCompanionNote   *string
	DepartureCompanionDays   map[string]bool
	DepartureBaseline        *PortalDepartureSnapshot
	PickupDays               map[string]bool
	BusDays                  map[string]bool
	Sick                     *bool
	SickSince                *time.Time
	Excused                  *bool
	ExcusedSince             *time.Time
	Status                   string
	EnrolledFrom             *calendar.Date
	EnrolledUntil            *calendar.Date
	PhotoPath                *string
	PhotoConsentGivenAt      *time.Time
	PhotoConsentGivenBy      *int64
	AGBAcceptedAt            *time.Time
	DataProcessingAcceptedAt *time.Time
	EmailContactAcceptedAt   *time.Time
	Person                   *Person
}

type PortalDepartureMode string
type PortalDepartureDays map[string]PortalDepartureMode
type PortalAllowedDepartureModes map[string][]PortalDepartureMode
type PortalDepartureSnapshot struct {
	DepartureDays         PortalDepartureDays
	AllowedDepartureModes PortalAllowedDepartureModes
	BusDays               map[string]bool
	PickupDays            map[string]bool
}

// CareEndedOn derives the inclusive care interval from the caller's frozen
// service day. It deliberately reads no process clock.
func (s *PortalStudent) CareEndedOn(day calendar.Date) bool {
	return s != nil && s.EnrolledUntil != nil && day.After(*s.EnrolledUntil)
}
