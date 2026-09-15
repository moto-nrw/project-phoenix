package domain

import "time"

// EnrollmentRecord is the student snapshot needed for enrollment decisions and their audits.
// Calendar dates use YYYY-MM-DD; absent dates are empty.
type EnrollmentRecord struct {
	ID                       int64
	CreatedAt                time.Time
	UpdatedAt                time.Time
	TenantID                 int64
	PersonID                 int64
	SchoolClass              string
	Status                   string
	EnrolledFrom             string
	EnrolledUntil            string
	GuardianName             *string
	GuardianContact          *string
	GuardianEmail            *string
	GuardianPhone            *string
	AddressStreet            *string
	AddressCity              *string
	AddressPostalCode        *string
	ExtraInfo                *string
	SupervisorNotes          *string
	HealthInfo               *string
	PickupStatus             *string
	DepartureCompanionNote   *string
	PhotoPath                *string
	GroupID                  *int64
	Sick                     *bool
	Excused                  *bool
	SickSince                *time.Time
	ExcusedSince             *time.Time
	PhotoConsentGivenAt      *time.Time
	AGBAcceptedAt            *time.Time
	DataProcessingAcceptedAt *time.Time
	EmailContactAcceptedAt   *time.Time
	PhotoConsentGivenBy      *int64
	AllowedDepartureModes    map[string][]string
	DepartureDays            map[string]string
	BusDays                  map[string]bool
	PickupDays               map[string]bool
}
