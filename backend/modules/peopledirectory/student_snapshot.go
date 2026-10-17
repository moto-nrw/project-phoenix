package peopledirectory

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// StudentSnapshot is the complete hydrated directory read value, including
// the original update baseline and nullable relations. Departure objects are
// typed JSON values, not ORM maps or foreign persistence types.
type StudentSnapshot struct {
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
	HealthInfo               *string
	PickupStatus             *string
	DepartureDays            json.RawMessage
	AllowedDepartureModes    json.RawMessage
	DepartureCompanionNote   *string
	DepartureCompanionDays   map[string]bool
	DepartureBaseline        *StudentDepartureSnapshot
	PickupDays               json.RawMessage
	BusDays                  json.RawMessage
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
	Person                   *DirectoryPersonSnapshot
}
type StudentDepartureSnapshot struct {
	DepartureDays         json.RawMessage
	AllowedDepartureModes json.RawMessage
	BusDays               json.RawMessage
	PickupDays            json.RawMessage
}

// StudentSnapshotReads preserves the existing query and mutation-lock paths.
type StudentSnapshotReads interface {
	StudentSnapshotByID(context.Context, int64) (*StudentSnapshot, error)
	StudentSnapshotForUpdate(context.Context, int64) (*StudentSnapshot, error)
}
