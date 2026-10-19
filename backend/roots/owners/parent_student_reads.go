package owners

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// NewParentStudentReads binds the SAME owner reads and mutation locks, with
// canonical hydration performed by its existing retained read before mapping.
func NewParentStudentReads(source peopledirectory.StudentSnapshotReads) parentportal.StudentReads {
	return parentportal.NewStudentReads(parentportal.StudentReadCallbacks{
		Find: func(ctx context.Context, id int64) (*parentportal.StudentSnapshot, error) {
			row, err := source.StudentSnapshotByID(ctx, id)
			if err != nil {
				return nil, err
			}
			return parentStudentSnapshot(row)
		},
		Lock: func(ctx context.Context, id int64) (*parentportal.StudentSnapshot, error) {
			row, err := source.StudentSnapshotForUpdate(ctx, id)
			if err != nil {
				return nil, err
			}
			return parentStudentSnapshot(row)
		},
	})
}

func parentStudentSnapshot(row *peopledirectory.StudentSnapshot) (*parentportal.StudentSnapshot, error) {
	if row == nil {
		return nil, nil
	}
	value := &parentportal.StudentSnapshot{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, PersonID: row.PersonID, SchoolClass: row.SchoolClass, GroupID: row.GroupID, AddressStreet: row.AddressStreet, AddressCity: row.AddressCity, AddressPostalCode: row.AddressPostalCode, ExtraInfo: row.ExtraInfo, SupervisorNotes: row.SupervisorNotes, HealthInfo: row.HealthInfo, PickupStatus: row.PickupStatus, DepartureCompanionNote: row.DepartureCompanionNote, DepartureCompanionDays: row.DepartureCompanionDays, Sick: row.Sick, SickSince: row.SickSince, Excused: row.Excused, ExcusedSince: row.ExcusedSince, Status: row.Status, EnrolledFrom: row.EnrolledFrom, EnrolledUntil: row.EnrolledUntil, PhotoPath: row.PhotoPath, PhotoConsentGivenAt: row.PhotoConsentGivenAt, PhotoConsentGivenBy: row.PhotoConsentGivenBy, AGBAcceptedAt: row.AGBAcceptedAt, DataProcessingAcceptedAt: row.DataProcessingAcceptedAt, EmailContactAcceptedAt: row.EmailContactAcceptedAt}
	// Only the four typed departure objects cross this JSON boundary. Other
	// scalar and nullable fields above are copied without revalidation.
	for _, field := range []struct {
		name   string
		raw    json.RawMessage
		target any
	}{
		{"DepartureDays", row.DepartureDays, &value.DepartureDays},
		{"AllowedDepartureModes", row.AllowedDepartureModes, &value.AllowedDepartureModes},
		{"PickupDays", row.PickupDays, &value.PickupDays},
		{"BusDays", row.BusDays, &value.BusDays},
	} {
		if field.raw != nil {
			if err := json.Unmarshal(field.raw, field.target); err != nil {
				return nil, fmt.Errorf("student snapshot: %s: %w", field.name, err)
			}
		}
	}
	if row.DepartureBaseline != nil {
		encoded, err := json.Marshal(row.DepartureBaseline)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(encoded, &value.DepartureBaseline); err != nil {
			return nil, err
		}
	}
	return parentportal.WithStudentPerson(value, parentPersonSnapshot(row.Person)), nil
}
