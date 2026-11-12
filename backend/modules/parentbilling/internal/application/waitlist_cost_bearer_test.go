package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// A registration confirmed with a cost bearer while every week waits keeps
// that cost bearer (#4699): the frozen contribution names it in a zero line,
// which is the only place the store reads it back from. When the week moves
// up, the cost bearer pays it, not the family. Without the zero line the
// read contribution has no cost bearer and the family pays the week.
func TestWaitlistedConfirmationKeepsItsCostBearerForTheMoveUp(t *testing.T) {
	t.Parallel()
	child := chainChild(1, 1, 4500)
	child.Bookings[0].Waitlisted = true
	bearer := youthOfficeBearer()

	frozen := computeHolidayContribution([]parentbilling.HolidayChild{child}, holidayRules{}, false, bearer)
	assert.Equal(t, int64(0), frozen.TotalCents, "a waiting week costs nothing")
	var named *int64
	for _, line := range frozen.Children[0].Lines {
		if line.Kind == parentbilling.LineCostTakeover {
			named = line.CostBearerID
			assert.Equal(t, int64(0), line.AmountCents)
		}
	}
	require.NotNil(t, named, "the frozen lines name the cost bearer although nothing is billed yet")

	// The store reads the cost bearer back from that line only.
	stored := parentbilling.HolidayContribution{
		Children: frozen.Children, Frozen: true, CostBearerID: named, CostBearerName: bearer.Name,
	}
	placed := chainChild(1, 1, 4500)
	placed.Bookings[0].Promoted = true
	member := siblingMember{
		registration: parentbilling.HolidayRegistration{ID: 9, Status: parentbilling.HolidayRegistrationConfirmed, Children: []parentbilling.HolidayChild{placed}},
		frozen:       &stored, movedUp: true,
	}
	moved := computeHolidaySiblings([]holidayMember{member.chainMember()}, holidayRules{})[0]
	assert.Equal(t, int64(0), moved.TotalCents, "the cost bearer pays the week that moved up")
	require.NotNil(t, moved.CostBearerID)
	assert.Equal(t, bearer.ID, *moved.CostBearerID)
}
