package schoolmembership

// ExternalCaregiverCandidate is one staff member the running-supervision
// directory looks at (#3823), with the People Directory facts it needs.
type ExternalCaregiverCandidate struct {
	StaffID int64
	IsGuest bool
	// HasPerson is false for a staff row whose person is missing.
	HasPerson bool
	// HasAccount is true when the person has a moto account.
	HasAccount bool
	FirstName  string
	LastName   string
}

// ExternalCaregiverOption is a currently valid external caregiver with the
// name the picker shows.
type ExternalCaregiverOption struct {
	StaffID  int64
	FullName string
}

// ActiveExternalCaregivers keeps the currently valid external caregivers:
// the candidates ExternalCaregiver accepts whose guest profile is active
// (activeGuestStaffIDs). They may help with a running supervision but never
// take over a group. Other staff without an account (an import without an
// e-mail address) stay out: nobody vouched for them as caregivers. The order
// is the caregiver picker's (SortActiveCaregivers), so both halves of the
// picker sort alike.
func ActiveExternalCaregivers(activeGuestStaffIDs []int64, candidates []ExternalCaregiverCandidate) []ExternalCaregiverOption {
	active := make(map[int64]struct{}, len(activeGuestStaffIDs))
	for _, staffID := range activeGuestStaffIDs {
		active[staffID] = struct{}{}
	}
	kept := make([]ActiveCaregiver, 0, len(candidates))
	for _, candidate := range candidates {
		if !ExternalCaregiver(candidate.IsGuest, candidate.HasPerson, candidate.HasAccount) {
			continue
		}
		if _, ok := active[candidate.StaffID]; ok {
			kept = append(kept, ActiveCaregiver{StaffID: candidate.StaffID, FirstName: candidate.FirstName, LastName: candidate.LastName})
		}
	}
	SortActiveCaregivers(kept)
	result := make([]ExternalCaregiverOption, 0, len(kept))
	for _, caregiver := range kept {
		result = append(result, ExternalCaregiverOption{StaffID: caregiver.StaffID, FullName: caregiver.FirstName + " " + caregiver.LastName})
	}
	return result
}
