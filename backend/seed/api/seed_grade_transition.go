package api

import (
	"context"
	"fmt"
	"time"
)

type seedGradeTransitionStep struct{}

func (seedGradeTransitionStep) Name() string { return "Seeding grade transition history" }

func (seedGradeTransitionStep) Run(_ context.Context, rt *Runtime) error {
	rt.Client.BindAuth(rt.TenantAuth)
	groupID := rt.FixedSeeder.groupIDs["sternengruppe"]
	if groupID == 0 {
		return fmt.Errorf("grade transition demo group not available")
	}
	if _, err := rt.Client.Post("/api/students", map[string]any{
		"first_name": "Alumni", "last_name": "Demo", "school_class": "Klasse 4z",
		"group_id": groupID, "birthday": "2015-06-01", "pickup_status": "self",
	}); err != nil {
		return fmt.Errorf("create graduation demo student: %w", err)
	}
	year := time.Now().In(seedBerlinLocation()).Year()
	raw, err := rt.Client.Post("/api/admin/grade-transitions/", map[string]any{
		"academic_year": fmt.Sprintf("%d-%d", year, year+1),
		"notes":         "Demo für den Schuljahreswechsel",
		"mappings": []map[string]any{
			{"from_class": "Klasse 1a", "to_class": "Klasse 2a"},
			{"from_class": "Klasse 4z", "to_class": nil},
		},
	})
	if err != nil {
		return fmt.Errorf("create grade transition: %w", err)
	}
	id, err := parseEnvelopeStringID(raw)
	if err != nil {
		return fmt.Errorf("parse grade transition: %w", err)
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/admin/grade-transitions/%d/apply", id), map[string]any{}); err != nil {
		return fmt.Errorf("apply grade transition: %w", err)
	}
	fmt.Println("  1 grade transition with history created")
	return nil
}
