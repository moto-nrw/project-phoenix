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
	signed := enrollment.SubmittedOfferingTerms{
		Name: "Ganztag", PriceCents: &price, DaysOfWeekMode: "fixed",
		AvailableDays: []string{"mon", "tue", "wed", "thu", "fri"}, ActivityGroupID: &groupID,
	}
	tests := []struct {
		name      string
		submitted *enrollment.SubmittedOfferingTerms
		current   func(*enrollment.SubmittedOfferingTerms)
		want      []string
	}{
		{"unchanged", &signed, func(*enrollment.SubmittedOfferingTerms) {}, nil},
		{"renamed and days reordered book the same", &signed, func(c *enrollment.SubmittedOfferingTerms) {
			c.Name = "Ganztagsbetreuung"
			c.AvailableDays = []string{"fri", "thu", "wed", "tue", "mon"}
		}, nil},
		{"no snapshot compares nothing", nil, func(c *enrollment.SubmittedOfferingTerms) { c.PriceCents = nil }, nil},
		{"price, days and activity", &signed, func(c *enrollment.SubmittedOfferingTerms) {
			c.PriceCents = &otherPrice
			c.AvailableDays = []string{"mon", "tue", "wed", "thu"}
			c.ActivityGroupID = &otherGroupID
		}, []string{enrollment.OfferingTermPrice, enrollment.OfferingTermDays, enrollment.OfferingTermActivity}},
		{"price removed", &signed, func(c *enrollment.SubmittedOfferingTerms) { c.PriceCents = nil }, []string{enrollment.OfferingTermPrice}},
		{"day mode", &signed, func(c *enrollment.SubmittedOfferingTerms) { c.DaysOfWeekMode = "parent_choice" }, []string{enrollment.OfferingTermDays}},
		{"activity unlinked", &signed, func(c *enrollment.SubmittedOfferingTerms) { c.ActivityGroupID = nil }, []string{enrollment.OfferingTermActivity}},
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
