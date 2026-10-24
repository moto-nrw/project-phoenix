package peopledirectory

import "encoding/json"

func StudentSnapshotFromState(row *StudentState) *StudentSnapshot {
	if row == nil {
		return nil
	}
	value := &StudentSnapshot{
		ID:                       row.ID,
		TenantID:                 row.TenantID,
		CreatedAt:                row.CreatedAt,
		UpdatedAt:                row.UpdatedAt,
		PersonID:                 row.PersonID,
		SchoolClass:              row.SchoolClass,
		GroupID:                  row.GroupID,
		AddressStreet:            row.AddressStreet,
		AddressCity:              row.AddressCity,
		AddressPostalCode:        row.AddressPostalCode,
		ExtraInfo:                row.ExtraInfo,
		HealthInfo:               row.HealthInfo,
		PickupStatus:             row.PickupStatus,
		DepartureCompanionNote:   row.DepartureCompanionNote,
		DepartureCompanionDays:   row.DepartureCompanionDays,
		Sick:                     row.Sick,
		SickSince:                row.SickSince,
		Excused:                  row.Excused,
		ExcusedSince:             row.ExcusedSince,
		Status:                   string(row.Status),
		EnrolledFrom:             row.EnrolledFrom,
		EnrolledUntil:            row.EnrolledUntil,
		PhotoPath:                row.PhotoPath,
		PhotoConsentGivenAt:      row.PhotoConsentGivenAt,
		PhotoConsentGivenBy:      row.PhotoConsentGivenBy,
		AGBAcceptedAt:            row.AGBAcceptedAt,
		DataProcessingAcceptedAt: row.DataProcessingAcceptedAt,
		EmailContactAcceptedAt:   row.EmailContactAcceptedAt,
	}
	if row.PickupDays != nil {
		value.PickupDays, _ = json.Marshal(row.PickupDays)
	}
	if row.AllowedDepartureModes != nil {
		value.AllowedDepartureModes, _ = json.Marshal(row.AllowedDepartureModes)
	}
	if row.DepartureDays != nil {
		value.DepartureDays, _ = json.Marshal(row.DepartureDays)
	}
	if row.BusDays != nil {
		value.BusDays, _ = json.Marshal(row.BusDays)
	}
	value.DepartureBaseline = studentDepartureStateSnapshot(row)
	value.Person = row.Person
	return value
}

func studentDepartureStateSnapshot(row *StudentState) *StudentDepartureSnapshot {
	if row.DepartureBaseline != nil {
		b := row.DepartureBaseline
		value := &StudentDepartureSnapshot{}
		if b.PickupDays != nil {
			value.PickupDays, _ = json.Marshal(b.PickupDays)
		}
		if b.AllowedDepartureModes != nil {
			value.AllowedDepartureModes, _ = json.Marshal(b.AllowedDepartureModes)
		}
		if b.DepartureDays != nil {
			value.DepartureDays, _ = json.Marshal(b.DepartureDays)
		}
		if b.BusDays != nil {
			value.BusDays, _ = json.Marshal(b.BusDays)
		}
		return value
	}
	return nil
}
