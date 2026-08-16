package parent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

func TestCareOfferingsResponseOmitsActivityGroups(t *testing.T) {
	payload, err := json.Marshal(toCareOfferingsResponse(&parentService.ChildCareOfferings{}))
	require.NoError(t, err)
	assert.NotContains(t, string(payload), `"groups"`)
}

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

func TestToCareOfferingsResponseUsesStableOfferingDiffValues(t *testing.T) {
	response := toCareOfferingsResponse(&parentService.ChildCareOfferings{
		PendingRequest: &parentService.PendingOfferingChange{
			Diff: []enrollmentService.OfferingChangeDiffEntry{{
				Label:    "Care club",
				OldState: "not_booked",
				NewState: "booked",
				NewDays:  []string{"mon", "wed"},
			}},
		},
	})

	require.NotNil(t, response.PendingRequest)
	require.Equal(t, []OfferingDiffResponse{{
		Label:    "Care club",
		OldState: "not_booked",
		OldDays:  []string{},
		NewState: "booked",
		NewDays:  []string{"mon", "wed"},
	}}, response.PendingRequest.Diff)
}
