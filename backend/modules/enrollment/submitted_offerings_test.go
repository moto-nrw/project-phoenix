package enrollment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

func TestChangedOfferingTerms(t *testing.T) {
	t.Parallel()
	price, otherPrice := 8000, 9500
	// Pure comparison: the activity ids are values, not database rows.
	var groupID, otherGroupID int64 = 3, 4
	signed := enrollment.OfferingTerms{
		Name: "Ganztag", PriceCents: &price, DaysOfWeekMode: "fixed",
		AvailableDays: []string{"mon", "tue", "wed", "thu", "fri"}, ActivityGroupID: &groupID,
	}
	parentChoice := signed
	parentChoice.DaysOfWeekMode = "parent_choice"
	tests := []struct {
		name      string
		submitted *enrollment.OfferingTerms
		current   func(*enrollment.OfferingTerms)
		want      []enrollment.OfferingTermChange
		picked    []string
	}{
		{"unchanged", &signed, func(*enrollment.OfferingTerms) {}, nil, nil},
		{"renamed and days reordered book the same", &signed, func(c *enrollment.OfferingTerms) {
			c.Name = "Ganztagsbetreuung"
			c.AvailableDays = []string{"fri", "thu", "wed", "tue", "mon"}
		}, nil, nil},
		{"no snapshot compares nothing", nil, func(c *enrollment.OfferingTerms) { c.PriceCents = nil }, nil, nil},
		{"price, days and activity", &signed, func(c *enrollment.OfferingTerms) {
			c.PriceCents = &otherPrice
			c.AvailableDays = []string{"mon", "tue", "wed", "thu"}
			c.ActivityGroupID = &otherGroupID
		}, []enrollment.OfferingTermChange{enrollment.OfferingTermPrice, enrollment.OfferingTermDays, enrollment.OfferingTermActivity}, nil},
		{"price removed", &signed, func(c *enrollment.OfferingTerms) { c.PriceCents = nil }, []enrollment.OfferingTermChange{enrollment.OfferingTermPrice}, nil},
		{"day mode", &signed, func(c *enrollment.OfferingTerms) { c.DaysOfWeekMode = "parent_choice" }, []enrollment.OfferingTermChange{enrollment.OfferingTermDays}, nil},
		{"activity unlinked", &signed, func(c *enrollment.OfferingTerms) { c.ActivityGroupID = nil }, []enrollment.OfferingTermChange{enrollment.OfferingTermActivity}, nil},
		{"parent choice keeps every picked day", &parentChoice, func(c *enrollment.OfferingTerms) {
			c.AvailableDays = []string{"mon", "wed"}
		}, nil, []string{"mon", "wed"}},
		{"parent choice loses a picked day", &parentChoice, func(c *enrollment.OfferingTerms) {
			c.AvailableDays = []string{"mon", "tue"}
		}, []enrollment.OfferingTermChange{enrollment.OfferingTermDays}, []string{"mon", "wed"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			current := *signedOrEmpty(tt.submitted)
			current.AvailableDays = append([]string(nil), current.AvailableDays...)
			tt.current(&current)
			assert.Equal(t, tt.want, enrollment.ChangedOfferingTerms(tt.submitted, current, tt.picked))
		})
	}
}

// signedOrEmpty is the catalog entry a case starts from: its snapshot, or an
// empty entry for the case without one.
func signedOrEmpty(terms *enrollment.OfferingTerms) *enrollment.OfferingTerms {
	if terms == nil {
		return &enrollment.OfferingTerms{}
	}
	copied := *terms
	return &copied
}
