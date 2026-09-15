package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
	peopleEnrollment "github.com/moto-nrw/project-phoenix/modules/peopledirectory/enrollment"
)

func (s *decisionService) readEnrollmentStudent(ctx context.Context, id int64, lock string) (*users.Student, error) {
	if s.StudentEnrollment == nil {
		return nil, errors.New("decision: student enrollment capability is required")
	}
	row, err := s.StudentEnrollment.ReadEnrollmentStudent(ctx, id, lock)
	if errors.Is(err, peopleEnrollment.ErrStudentNotFound) {
		return nil, fmt.Errorf("%w: %w", sql.ErrNoRows, err)
	}
	if err != nil {
		return nil, err
	}
	student := &users.Student{
		PersonID:                 row.PersonID,
		SchoolClass:              row.SchoolClass,
		GuardianName:             row.GuardianName,
		GuardianContact:          row.GuardianContact,
		GuardianEmail:            row.GuardianEmail,
		GuardianPhone:            row.GuardianPhone,
		AddressStreet:            row.AddressStreet,
		AddressCity:              row.AddressCity,
		AddressPostalCode:        row.AddressPostalCode,
		ExtraInfo:                row.ExtraInfo,
		SupervisorNotes:          row.SupervisorNotes,
		HealthInfo:               row.HealthInfo,
		PickupStatus:             row.PickupStatus,
		DepartureCompanionNote:   row.DepartureCompanionNote,
		PhotoPath:                row.PhotoPath,
		GroupID:                  row.GroupID,
		Sick:                     row.Sick,
		Excused:                  row.Excused,
		SickSince:                row.SickSince,
		ExcusedSince:             row.ExcusedSince,
		PhotoConsentGivenAt:      row.PhotoConsentGivenAt,
		AGBAcceptedAt:            row.AGBAcceptedAt,
		DataProcessingAcceptedAt: row.DataProcessingAcceptedAt,
		EmailContactAcceptedAt:   row.EmailContactAcceptedAt,
		PhotoConsentGivenBy:      row.PhotoConsentGivenBy,
		Status:                   users.StudentStatus(row.Status),
	}
	student.ID, student.TenantID = row.ID, row.TenantID
	student.CreatedAt, student.UpdatedAt = row.CreatedAt, row.UpdatedAt
	for _, value := range []struct {
		raw    string
		target **timezone.Date
	}{{row.EnrolledFrom, &student.EnrolledFrom}, {row.EnrolledUntil, &student.EnrolledUntil}} {
		if value.raw != "" {
			date, err := timezone.ParseDate(value.raw)
			if err != nil {
				return nil, fmt.Errorf("decision: invalid owner enrollment date: %w", err)
			}
			*value.target = &date
		}
	}
	allowed := users.AllowedDepartureModes{}
	for day, modes := range row.AllowedDepartureModes {
		for _, mode := range modes {
			allowed[day] = append(allowed[day], users.DepartureMode(mode))
		}
	}
	departure := users.DepartureDays{}
	for day, mode := range row.DepartureDays {
		departure[day] = users.DepartureMode(mode)
	}
	// Preserve the legacy hydration precedence for rows written before backfill.
	allowed = allowed.Normalize()
	if !allowed.HasAny() {
		if normalized := departure.Normalize(); normalized.HasAny() {
			allowed = users.AllowedDepartureModesFromDeparture(normalized)
		} else {
			allowed = users.AllowedDepartureModesFromLegacy(users.BusDays(row.BusDays).Normalize(), users.PickupDays(row.PickupDays).Normalize())
		}
	}
	student.AllowedDepartureModes = allowed
	student.DepartureDays = allowed.DepartureDays()
	student.BusDays = allowed.BusDays()
	student.PickupDays = allowed.PickupDays()
	student.SnapshotDeparturePlan()
	return student, nil
}
