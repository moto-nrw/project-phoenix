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
	tests := []struct {
		name      string
		submitted *enrollment.OfferingTerms
		current   func(*enrollment.OfferingTerms)
		want      []enrollment.OfferingTermChange
	}{
		{"unchanged", &signed, func(*enrollment.OfferingTerms) {}, nil},
		{"renamed and days reordered book the same", &signed, func(c *enrollment.OfferingTerms) {
			c.Name = "Ganztagsbetreuung"
			c.AvailableDays = []string{"fri", "thu", "wed", "tue", "mon"}
		}, nil},
		{"no snapshot compares nothing", nil, func(c *enrollment.OfferingTerms) { c.PriceCents = nil }, nil},
		{"price, days and activity", &signed, func(c *enrollment.OfferingTerms) {
			c.PriceCents = &otherPrice
			c.AvailableDays = []string{"mon", "tue", "wed", "thu"}
			c.ActivityGroupID = &otherGroupID
		}, []enrollment.OfferingTermChange{enrollment.OfferingTermPrice, enrollment.OfferingTermDays, enrollment.OfferingTermActivity}},
		{"price removed", &signed, func(c *enrollment.OfferingTerms) { c.PriceCents = nil }, []enrollment.OfferingTermChange{enrollment.OfferingTermPrice}},
		{"day mode", &signed, func(c *enrollment.OfferingTerms) { c.DaysOfWeekMode = "parent_choice" }, []enrollment.OfferingTermChange{enrollment.OfferingTermDays}},
		{"activity unlinked", &signed, func(c *enrollment.OfferingTerms) { c.ActivityGroupID = nil }, []enrollment.OfferingTermChange{enrollment.OfferingTermActivity}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			current := signed
			current.AvailableDays = append([]string(nil), signed.AvailableDays...)
			tt.current(&current)
			assert.Equal(t, tt.want, enrollment.ChangedOfferingTerms(tt.submitted, current))
		})
	}
}
