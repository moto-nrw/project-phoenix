package domain

func ApplyEnrollmentPatch(record *StudentRecord, patch EnrollmentProfilePatch) {
	if patch.ExtraInfoSet {
		record.ExtraInfo = patch.ExtraInfo
	}
	if patch.HealthInfoSet {
		record.HealthInfo = patch.HealthInfo
	}
	if patch.SupervisorNotesSet {
		record.SupervisorNotes = patch.SupervisorNotes
	}
	if patch.GroupIDSet {
		record.GroupID = patch.GroupID
	}
	if patch.AddressSet {
		record.AddressStreet, record.AddressCity, record.AddressPostalCode = patch.AddressStreet, patch.AddressCity, patch.AddressPostalCode
	}
	if patch.PhotoConsentGivenAtSet {
		record.PhotoConsentGivenAt = patch.PhotoConsentGivenAt
	}
	if patch.PhotoConsentGivenBySet {
		record.PhotoConsentGivenBy = patch.PhotoConsentGivenBy
	}
	if patch.AGBAcceptedAtSet {
		record.AGBAcceptedAt = patch.AGBAcceptedAt
	}
	if patch.DataProcessingAcceptedAtSet {
		record.DataProcessingAcceptedAt = patch.DataProcessingAcceptedAt
	}
	if patch.EmailContactAcceptedAtSet {
		record.EmailContactAcceptedAt = patch.EmailContactAcceptedAt
	}
	applyInitialDeparture(record, patch)
}

func applyInitialDeparture(record *StudentRecord, patch EnrollmentProfilePatch) {
	if patch.DepartureSet {
		record.AllowedDepartureModes = make(AllowedDepartureModes, len(patch.AllowedDepartureModes))
		for day, modes := range patch.AllowedDepartureModes {
			for _, mode := range modes {
				record.AllowedDepartureModes[day] = append(record.AllowedDepartureModes[day], DepartureMode(mode))
			}
		}
		record.DepartureDays = make(DepartureDays, len(patch.DepartureDays))
		for day, mode := range patch.DepartureDays {
			record.DepartureDays[day] = DepartureMode(mode)
		}
		record.BusDays, record.PickupDays = patch.BusDays, patch.PickupDays
		record.PickupStatus, record.DepartureCompanionNote = &patch.PickupStatus, patch.DepartureCompanionNote
	}
}
