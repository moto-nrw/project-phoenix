package api

import (
	"context"

	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// caregiverChainsFromMembership is the caregiver half of the account
// listings, served from the School Membership capability instead of the auth
// repository's former staff/teacher join.
func caregiverChainsFromMembership(membership schoolMembershipCompose.StaffLookup) caregiverChainQuery {
	return membershipCaregiverChains{membership: membership}
}

type membershipCaregiverChains struct {
	membership schoolMembershipCompose.StaffLookup
}

// CaregiverChainByPersonIDs returns the live staff record (and its live
// teacher record, when present) behind each person, keyed by person ID.
// Persons without a live staff record are absent from the result; a person
// with several staff rows keeps the one with the lowest staff ID, as the
// former "ORDER BY person_id ASC, staff id ASC" query did.
func (r membershipCaregiverChains) CaregiverChainByPersonIDs(ctx context.Context, personIDs []int64) (map[int64]CaregiverChain, error) {
	chains, err := schoolmembership.StaffChainsByPerson(ctx, r.membership, personIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]CaregiverChain, len(chains))
	for personID, chain := range chains {
		result[personID] = CaregiverChain(chain)
	}
	return result, nil
}
