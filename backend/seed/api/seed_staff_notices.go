package api

import (
	"context"
	"fmt"
)

// seedStaffNoticesStep creates notices that make the Tagesinformationen page
// usable in the demo environment, including one acknowledgement and one
// recurring notice.
type seedStaffNoticesStep struct{}

func (seedStaffNoticesStep) Name() string { return "Seeding staff notices" }

func (seedStaffNoticesStep) Run(_ context.Context, rt *Runtime) error {
	today := todaySeedDate()
	isoWeekday := int16((int(today.Weekday())+6)%7 + 1)
	notices := []map[string]any{
		{
			"title":                    "Räumungsübung heute",
			"body":                     "Bitte begleiten Sie Ihre Gruppe zum Sammelplatz.",
			"priority":                 "important",
			"valid_from":               today.String(),
			"weekdays":                 []int16{},
			"week_pattern":             0,
			"requires_acknowledgement": true,
			"active":                   true,
		},
		{
			"title":                    "Turnhalle belegt",
			"body":                     "Die Turnhalle ist heute bis 15 Uhr belegt.",
			"priority":                 "info",
			"valid_from":               today.String(),
			"weekdays":                 []int16{isoWeekday},
			"week_pattern":             0,
			"requires_acknowledgement": false,
			"active":                   true,
		},
	}

	created := 0
	for index, notice := range notices {
		raw, err := rt.Client.Post("/api/staff-notices/", notice)
		if err != nil {
			return fmt.Errorf("create staff notice %q: %w", notice["title"], err)
		}
		created++

		if index != 0 {
			continue
		}
		id, err := parseEnvelopeStringID(raw)
		if err != nil {
			return fmt.Errorf("parse staff notice response: %w", err)
		}
		if _, err := rt.Client.Post(fmt.Sprintf("/api/staff-notices/%d/acknowledge", id), nil); err != nil {
			return fmt.Errorf("acknowledge staff notice: %w", err)
		}
	}

	fmt.Printf("  %d staff notices created, 1 acknowledged\n", created)
	return nil
}
