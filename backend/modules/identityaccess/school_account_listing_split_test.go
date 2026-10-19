package identityaccess_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

type listingRow struct {
	key     identityaccess.ListedAccountKey
	pending bool
	name    string
}

func (r listingRow) isPending() bool                             { return r.pending }
func (r listingRow) listingKey() identityaccess.ListedAccountKey { return r.key }

// The account rows are enriched and ordered; pending invitations follow in
// their read order without being enriched.
func TestOrderSchoolAccountListingAppendsPendingInvitations(t *testing.T) {
	t.Parallel()
	rows := []listingRow{
		{name: "invite-1", pending: true},
		{name: "zorn", key: identityaccess.ListedAccountKey{LastName: "Zorn"}},
		{name: "invite-2", pending: true},
		{name: "abel", key: identityaccess.ListedAccountKey{LastName: "Abel"}},
	}
	var enriched []string
	enrich := func(accounts []listingRow) error {
		for index := range accounts {
			enriched = append(enriched, accounts[index].name)
			accounts[index].key.HasPerson = accounts[index].name == "zorn"
		}
		return nil
	}
	ordered, err := identityaccess.OrderSchoolAccountListing(rows, listingRow.isPending, enrich, listingRow.listingKey)
	require.NoError(t, err)
	assert.Equal(t, []string{"zorn", "abel"}, enriched)
	names := make([]string, 0, len(ordered))
	for _, row := range ordered {
		names = append(names, row.name)
	}
	assert.Equal(t, []string{"zorn", "abel", "invite-1", "invite-2"}, names, "the enriched person fact orders zorn first")
}

// Across schools the account rows group by school ID; a failed enrichment
// fails the listing.
func TestOrderAccountListingBySchoolID(t *testing.T) {
	t.Parallel()
	rows := []listingRow{
		{name: "invite", pending: true},
		{name: "b-abel", key: identityaccess.ListedAccountKey{SchoolID: listingSchoolB, HasPerson: true, LastName: "Abel"}},
		{name: "a-zorn", key: identityaccess.ListedAccountKey{SchoolID: listingSchoolA, HasPerson: true, LastName: "Zorn"}},
	}
	ordered, err := identityaccess.OrderAccountListingBySchoolID(rows, listingRow.isPending, func([]listingRow) error { return nil }, listingRow.listingKey)
	require.NoError(t, err)
	assert.Equal(t, []listingRow{rows[2], rows[1], rows[0]}, ordered)

	failure := errors.New("persons unavailable")
	_, err = identityaccess.OrderAccountListingBySchoolID(rows, listingRow.isPending, func([]listingRow) error { return failure }, listingRow.listingKey)
	assert.ErrorIs(t, err, failure)
}
