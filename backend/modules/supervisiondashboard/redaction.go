package supervisiondashboard

// The dashboard redacts by permission: sections a caller may not read stay
// in the projection, emptied, instead of failing the request.

func redactPlannedPickupTimes(instances []PlannedInstance) {
	for i := range instances {
		instances[i].PickupTimesLoaded = false
		instances[i].PickupTimesRedacted = true
		for j := range instances[i].RosterPreview {
			instances[i].RosterPreview[j].PickupTime = nil
		}
	}
}

// redactAbsenceStatuses clears sick and excused status with their timestamps.
// They need users:read like in the child list and group view (ADR 0090); a
// guest reads the dashboard with groups:read only.
func redactAbsenceStatuses(projection *Projection) {
	redact := func(visit *Visit) {
		visit.Sick, visit.SickSince = false, nil
		visit.Excused, visit.ExcusedSince = false, nil
	}
	for i := range projection.Visits {
		redact(&projection.Visits[i])
	}
	for i := range projection.OpenRooms {
		for j := range projection.OpenRooms[i].Students {
			redact(&projection.OpenRooms[i].Students[j].Visit)
		}
	}
}
