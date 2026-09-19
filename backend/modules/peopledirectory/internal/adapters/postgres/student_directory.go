package postgres

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/postgres/calendar"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// studentRecordRow is the whole users.students row. The departure plans are
// selected in the same statement as everything else: the retained repository
// needed a second query for them because its generic list could not reach
// scan-only columns, and one statement is both cheaper and atomic.
type studentRecordRow struct {
	ID        int64     `bun:"id"`
	CreatedAt time.Time `bun:"created_at"`
	UpdatedAt time.Time `bun:"updated_at"`
	TenantID  int64     `bun:"tenant_id"`

	PersonID    int64  `bun:"person_id"`
	SchoolClass string `bun:"school_class"`
	GroupID     *int64 `bun:"group_id"`
	Status      string `bun:"status"`

	EnrolledFrom  *calendar.Date `bun:"enrolled_from"`
	EnrolledUntil *calendar.Date `bun:"enrolled_until"`

	AddressStreet     *string `bun:"address_street"`
	AddressCity       *string `bun:"address_city"`
	AddressPostalCode *string `bun:"address_postal_code"`

	ExtraInfo       *string `bun:"extra_info"`
	SupervisorNotes *string `bun:"supervisor_notes"`
	HealthInfo      *string `bun:"health_info"`
	PickupStatus    *string `bun:"pickup_status"`

	DepartureDays          domain.DepartureDays         `bun:"departure_days"`
	AllowedDepartureModes  domain.AllowedDepartureModes `bun:"allowed_departure_modes"`
	PickupDays             domain.PickupDays            `bun:"pickup_days"`
	BusDays                domain.BusDays               `bun:"bus_days"`
	DepartureCompanionNote *string                      `bun:"departure_companion_note"`

	Sick         *bool      `bun:"sick"`
	SickSince    *time.Time `bun:"sick_since"`
	Excused      *bool      `bun:"excused"`
	ExcusedSince *time.Time `bun:"excused_since"`

	PhotoPath           *string    `bun:"photo_path"`
	PhotoConsentGivenAt *time.Time `bun:"photo_consent_given_at"`
	PhotoConsentGivenBy *int64     `bun:"photo_consent_given_by"`

	AGBAcceptedAt            *time.Time `bun:"agb_accepted_at"`
	DataProcessingAcceptedAt *time.Time `bun:"data_processing_accepted_at"`
	EmailContactAcceptedAt   *time.Time `bun:"email_contact_accepted_at"`
}

func (r studentRecordRow) toDomain() domain.StudentRecord {
	record := domain.StudentRecord{
		ID: r.ID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, TenantID: r.TenantID,
		PersonID: r.PersonID, SchoolClass: r.SchoolClass, GroupID: r.GroupID, Status: r.Status,

		AddressStreet: r.AddressStreet, AddressCity: r.AddressCity, AddressPostalCode: r.AddressPostalCode,
		ExtraInfo: r.ExtraInfo, SupervisorNotes: r.SupervisorNotes,
		HealthInfo: r.HealthInfo, PickupStatus: r.PickupStatus,
		DepartureDays: r.DepartureDays, AllowedDepartureModes: r.AllowedDepartureModes,
		PickupDays: r.PickupDays, BusDays: r.BusDays, DepartureCompanionNote: r.DepartureCompanionNote,
		Sick: r.Sick, SickSince: r.SickSince, Excused: r.Excused, ExcusedSince: r.ExcusedSince,
		PhotoPath: r.PhotoPath, PhotoConsentGivenAt: r.PhotoConsentGivenAt, PhotoConsentGivenBy: r.PhotoConsentGivenBy,
		AGBAcceptedAt: r.AGBAcceptedAt, DataProcessingAcceptedAt: r.DataProcessingAcceptedAt,
		EmailContactAcceptedAt: r.EmailContactAcceptedAt,
	}
	if r.EnrolledFrom != nil {
		record.EnrolledFrom = r.EnrolledFrom.String()
	}
	if r.EnrolledUntil != nil {
		record.EnrolledUntil = r.EnrolledUntil.String()
	}
	return record
}
