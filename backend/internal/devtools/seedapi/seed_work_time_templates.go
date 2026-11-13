package api

import (
	"encoding/json"
	"fmt"
	"time"
)

// seedWorkTimeTemplateAssignees is how many staff members get the full-time
// template, so the catalog page (#6891) shows a template in use.
const seedWorkTimeTemplateAssignees = 3

type seedWorkTimeTemplate struct {
	name     string
	rotation int
	// minutes per (week, weekday); a missing weekday has no Soll.
	weeks    [][5]int
	archived bool
}

// seedWorkTimeTemplates fills the school's Arbeitszeitmodell-Vorlagen
// (#6891): one in use, one unused A/B pattern and one the school no longer
// offers. The full-time template carries the same Soll as the custom
// schedules, so assigning it changes no time account.
func seedWorkTimeTemplates() []seedWorkTimeTemplate {
	day := timeTrackingDailyTargetMin
	return []seedWorkTimeTemplate{
		{name: "Vollzeit 40 Stunden", rotation: 1, weeks: [][5]int{{day, day, day, day, day}}},
		{name: "Teilzeit A/B-Woche", rotation: 2, weeks: [][5]int{{240, 240, 240, 240, 240}, {240, 240, 240, 0, 0}}},
		{name: "Teilzeit 30 Stunden (alt)", rotation: 1, weeks: [][5]int{{360, 360, 360, 360, 360}}, archived: true},
	}
}

// seedSollTargets writes every staff member's own schedule from validFrom on
// and then the school's templates, which take over some of them from today.
func seedSollTargets(rt *Runtime, staff []StaffCredentials, staffIDByEmail map[string]int64, validFrom time.Time) (int, error) {
	count, err := seedSchedulesViaAPI(rt, staff, staffIDByEmail, validFrom)
	if err != nil {
		return count, err
	}
	return count, seedWorkTimeTemplatesViaAPI(rt, staff, staffIDByEmail, validFrom)
}

func seedWorkTimeTemplatesViaAPI(rt *Runtime, staff []StaffCredentials, staffIDByEmail map[string]int64, anchor time.Time) error {
	currentAuth := rt.Client.auth
	defer rt.Client.BindAuth(currentAuth)
	rt.Client.BindAuth(rt.TenantAuth)

	ids := make([]int64, 0, len(seedWorkTimeTemplates()))
	for _, template := range seedWorkTimeTemplates() {
		id, err := postSeedWorkTimeTemplate(rt, template, toDateKey(anchor))
		if err != nil {
			return err
		}
		if template.archived {
			if _, err := rt.Client.Post(fmt.Sprintf("/api/work-time-models/%d/archive", id), nil); err != nil {
				return fmt.Errorf("archive work time template %q: %w", template.name, err)
			}
		}
		ids = append(ids, id)
	}

	return assignSeedWorkTimeTemplate(rt, staff, staffIDByEmail, ids[0])
}

// assignSeedWorkTimeTemplate binds the first staff members that have a Soll
// of their own to the template.
func assignSeedWorkTimeTemplate(rt *Runtime, staff []StaffCredentials, staffIDByEmail map[string]int64, modelID int64) error {
	assigned := 0
	for _, cred := range staff {
		if assigned == seedWorkTimeTemplateAssignees {
			return nil
		}
		if cred.Position == "Extern" || cred.Name == seedStaffWithoutTarget {
			continue
		}
		staffID := staffIDByEmail[cred.Email]
		if _, err := rt.Client.Put(fmt.Sprintf("/api/staff/%d/schedule", staffID), map[string]any{
			"mode": "template", "model_id": modelID,
		}); err != nil {
			return fmt.Errorf("assign work time template to staff %d: %w", staffID, err)
		}
		assigned++
	}
	return nil
}

func postSeedWorkTimeTemplate(rt *Runtime, template seedWorkTimeTemplate, anchor string) (int64, error) {
	entries := make([]map[string]any, 0, template.rotation*5)
	for week, days := range template.weeks {
		for weekday, minutes := range days {
			if minutes > 0 {
				entries = append(entries, map[string]any{"week_index": week, "day_of_week": weekday, "target_minutes": minutes})
			}
		}
	}
	resp, err := rt.Client.Post("/api/work-time-models", map[string]any{
		"name": template.name, "rotation_length": template.rotation, "rotation_anchor_date": anchor, "entries": entries,
	})
	if err != nil {
		return 0, fmt.Errorf("post work time template %q: %w", template.name, err)
	}
	var payload struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &payload); err != nil {
		return 0, fmt.Errorf("parse work time template response: %w", err)
	}
	if payload.Data.ID == 0 {
		return 0, fmt.Errorf("work time template %q: response carries no id", template.name)
	}
	return payload.Data.ID, nil
}
