package parent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

func TestToCareOfferingsResponseIncludesOfferingInterval(t *testing.T) {
	starts := timezone.NewDate(2026, 10, 1)
	endsExclusive := timezone.NewDate(2027, 2, 1)

	response := toCareOfferingsResponse(&parentService.ChildCareOfferings{
		Offerings: []parentService.CareOfferingSelection{{
			OfferingID:  42,
			Name:        "Spätbetreuung",
			ValidFrom:   &starts,
			ValidUntil:  &endsExclusive,
			StartsLater: true,
		}},
	})

	require.Len(t, response.Offerings, 1)
	item := response.Offerings[0]
	assert.Equal(t, "2026-10-01", item.ValidFrom)
	assert.Equal(t, "2027-01-31", item.ValidUntil)
	assert.True(t, item.StartsLater)
}
