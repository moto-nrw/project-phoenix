package schoolmembership

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActiveExternalCaregivers(t *testing.T) {
	t.Parallel()
	zoe, oezlem, otto, accountHolder, inactive, notGuest, noPerson := int64(81), int64(82), int64(83), int64(84), int64(85), int64(86), int64(87)
	candidates := []ExternalCaregiverCandidate{
		{StaffID: zoe, IsGuest: true, HasPerson: true, FirstName: "Zoe", LastName: "Albers"},
		{StaffID: oezlem, IsGuest: true, HasPerson: true, InvitationPending: true, FirstName: "Özlem", LastName: "Kaya"},
		{StaffID: otto, IsGuest: true, HasPerson: true, FirstName: "otto", LastName: "Berg"},
		{StaffID: accountHolder, IsGuest: true, HasPerson: true, HasAccount: true, FirstName: "Anna", LastName: "Konto"},
		{StaffID: inactive, IsGuest: true, HasPerson: true, FirstName: "Anton", LastName: "Alt"},
		{StaffID: notGuest, HasPerson: true, FirstName: "Bea", LastName: "Import"},
		{StaffID: noPerson, IsGuest: true},
	}
	active := []int64{zoe, oezlem, otto, accountHolder, notGuest, noPerson}

	// Every active external is listed (#4573); only those with an account or
	// an open invitation may be picked for a new assignment.
	assert.Equal(t, []ExternalCaregiverOption{
		{StaffID: accountHolder, FullName: "Anna Konto", Assignable: true, AccountStatus: ExternalAccountStatusAccount},
		{StaffID: otto, FullName: "otto Berg", AccountStatus: ExternalAccountStatusNone},
		{StaffID: oezlem, FullName: "Özlem Kaya", Assignable: true, AccountStatus: ExternalAccountStatusInvitationPending},
		{StaffID: zoe, FullName: "Zoe Albers", AccountStatus: ExternalAccountStatusNone},
	}, ActiveExternalCaregivers(active, candidates))
	assert.Empty(t, ActiveExternalCaregivers(nil, candidates))
}
