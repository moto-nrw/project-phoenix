package compose

import (
	"testing"

	reviewbindings "github.com/moto-nrw/project-phoenix/roots/requestreviewbinding"

	"github.com/moto-nrw/project-phoenix/modules/careplan/offeringrequests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToOfferingRequestResponse_IncludesRemainingDaysForOverridePreview(t *testing.T) {
	t.Parallel()

	view := &offeringrequests.ReviewItem{
		Request: &offeringrequests.Request{},
		Diff: []offeringrequests.DiffEntry{{
			OfferingID:          9,
			Label:               "Mittagessen",
			NewState:            "booked",
			NewDays:             []string{"mon", "tue", "wed"},
			NewAutomaticDays:    []string{"tue", "wed"},
			NewRuleDays:         []string{"tue"},
			NewDaysWithoutRules: []string{"mon", "wed"},
			AutoTriggerIDs:      []int64{5},
			AutoTriggerNames:    []string{"Randstunde"},
		}},
	}

	response := reviewbindings.ToOfferingRequestResponse(view)

	require.Len(t, response.Diff, 1)
	assert.Equal(t, "Di", response.Diff[0].RuleDays)
	assert.Equal(t, "Mo, Mi", response.Diff[0].NewWhenExcluded)
	assert.True(t, response.Diff[0].Optoutable)
}

func TestToOfferingRequestResponse_OmitsFullWithdrawalForAnOrdinaryRequest(t *testing.T) {
	t.Parallel()

	view := &offeringrequests.ReviewItem{
		Request: &offeringrequests.Request{},
		Diff: []offeringrequests.DiffEntry{{
			OfferingID: 3,
			Label:      "Regelbetreuung",
			OldState:   "booked",
			OldDays:    []string{"mon", "tue"},
			NewState:   "booked",
			NewDays:    []string{"mon"},
		}},
	}

	resp := reviewbindings.ToOfferingRequestResponse(view)

	assert.False(t, resp.FullWithdrawal)
	assert.Empty(t, resp.Unchanged)
}

func TestToOfferingRequestResponse_ReportsFullWithdrawalAndUntouchedBookings(t *testing.T) {
	t.Parallel()

	view := &offeringrequests.ReviewItem{
		Request:        &offeringrequests.Request{},
		FullWithdrawal: true,
		Diff: []offeringrequests.DiffEntry{{
			OfferingID: 3,
			Label:      "Regelbetreuung",
			OldState:   "booked",
			OldDays:    []string{"mon", "tue"},
			NewState:   "removed",
		}},
		Unchanged: []offeringrequests.DiffEntry{{
			OfferingID: 4,
			Label:      "Mittagessen",
			OldState:   "booked",
			OldDays:    []string{"mon"},
			NewState:   "booked",
			NewDays:    []string{"mon"},
		}},
	}

	resp := reviewbindings.ToOfferingRequestResponse(view)

	assert.True(t, resp.FullWithdrawal)
	assert.Equal(t, "abgemeldet", resp.Diff[0].New)
	require.Len(t, resp.Unchanged, 1)
	assert.Equal(t, "4", resp.Unchanged[0].OfferingID)
	assert.Equal(t, "Mittagessen", resp.Unchanged[0].Label)
	assert.Equal(t, "Mo", resp.Unchanged[0].Days)
}
