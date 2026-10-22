package schoolmembership

// The account status of an external caregiver (#4573), as the staff and
// supervision responses carry it.
const (
	// ExternalAccountStatusAccount: the person has a moto account.
	ExternalAccountStatusAccount = "account"
	// ExternalAccountStatusInvitationPending: no account yet, but an unused,
	// unexpired invitation names the person.
	ExternalAccountStatusInvitationPending = "invitation_pending"
	// ExternalAccountStatusNone: neither an account nor an open invitation.
	ExternalAccountStatusNone = "none"
)

// ExternalAccountStatus names an external caregiver's account status.
func ExternalAccountStatus(hasAccount, invitationPending bool) string {
	switch {
	case hasAccount:
		return ExternalAccountStatusAccount
	case invitationPending:
		return ExternalAccountStatusInvitationPending
	default:
		return ExternalAccountStatusNone
	}
}

// ExternalCaregiverAssignable reports whether an external caregiver may be
// named in a new assignment (#4573): only with an account or an open
// invitation, so somebody vouched for the person with an address. Existing
// assignments are never re-checked.
func ExternalCaregiverAssignable(hasAccount, invitationPending bool) bool {
	return hasAccount || invitationPending
}

// ExternalCaregiverCandidate is one staff member the running-supervision
// directory looks at (#3823), with the People Directory and Identity & Access
// facts it needs.
type ExternalCaregiverCandidate struct {
	StaffID int64
	IsGuest bool
	// HasPerson is false for a staff row whose person is missing.
	HasPerson bool
	// HasAccount is true when the person has a moto account.
	HasAccount bool
	// InvitationPending is true when an open invitation names the person.
	InvitationPending bool
	FirstName         string
	LastName          string
}

// ExternalCaregiverOption is a currently valid external caregiver with the
// name the picker shows. Assignable tells whether a new assignment may name
// the person (ExternalCaregiverAssignable).
type ExternalCaregiverOption struct {
	StaffID       int64
	FullName      string
	Assignable    bool
	AccountStatus string
}

// ActiveExternalCaregivers keeps the currently valid external caregivers:
// the candidates ExternalCaregiver accepts whose guest profile is active
// (activeGuestStaffIDs), with or without an account. They may help with a
// running supervision but never take over a group. Other staff without a
// guest profile stay out. The order is the caregiver picker's
// (SortActiveCaregivers), so both halves of the picker sort alike.
func ActiveExternalCaregivers(activeGuestStaffIDs []int64, candidates []ExternalCaregiverCandidate) []ExternalCaregiverOption {
	active := make(map[int64]struct{}, len(activeGuestStaffIDs))
	for _, staffID := range activeGuestStaffIDs {
		active[staffID] = struct{}{}
	}
	kept := make([]ActiveCaregiver, 0, len(candidates))
	byStaffID := make(map[int64]ExternalCaregiverCandidate, len(candidates))
	for _, candidate := range candidates {
		if !ExternalCaregiver(candidate.IsGuest, candidate.HasPerson) {
			continue
		}
		if _, ok := active[candidate.StaffID]; ok {
			kept = append(kept, ActiveCaregiver{StaffID: candidate.StaffID, FirstName: candidate.FirstName, LastName: candidate.LastName})
			byStaffID[candidate.StaffID] = candidate
		}
	}
	SortActiveCaregivers(kept)
	result := make([]ExternalCaregiverOption, 0, len(kept))
	for _, caregiver := range kept {
		candidate := byStaffID[caregiver.StaffID]
		result = append(result, ExternalCaregiverOption{
			StaffID: caregiver.StaffID, FullName: caregiver.FirstName + " " + caregiver.LastName,
			Assignable:    ExternalCaregiverAssignable(candidate.HasAccount, candidate.InvitationPending),
			AccountStatus: ExternalAccountStatus(candidate.HasAccount, candidate.InvitationPending),
		})
	}
	return result
}
