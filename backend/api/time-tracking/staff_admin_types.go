package timetracking

import (
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// ScheduleEntryRequest represents a single day in the schedule
type ScheduleEntryRequest struct {
	WeekIndex     int     `json:"week_index"`
	DayOfWeek     int     `json:"day_of_week"`
	TargetMinutes int     `json:"target_minutes"`
	StartTime     *string `json:"start_time,omitempty"`
}

// ScheduleEntryResponse represents a single day in the schedule response
type ScheduleEntryResponse struct {
	WeekIndex     int     `json:"week_index"`
	DayOfWeek     int     `json:"day_of_week"`
	TargetMinutes int     `json:"target_minutes"`
	StartTime     *string `json:"start_time,omitempty"`
}

// ScheduleModelInfo describes the assigned work-time template, when there is one.
type ScheduleModelInfo struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	RotationLength     int    `json:"rotation_length"`
	RotationAnchorDate string `json:"rotation_anchor_date"`
}

// ScheduleResponse wraps the resolved schedule with rotation metadata.
type ScheduleResponse struct {
	Mode               string                  `json:"mode"`
	Model              *ScheduleModelInfo      `json:"model,omitempty"`
	RotationLength     int                     `json:"rotation_length"`
	RotationAnchorDate string                  `json:"rotation_anchor_date"`
	Entries            []ScheduleEntryResponse `json:"entries"`
	WeeklyTotals       []int                   `json:"weekly_totals"`
	ValidFrom          string                  `json:"valid_from,omitempty"`
}

// scheduleUpdateRequest is the union body for PUT /api/staff/{id}/schedule.
//
// Mode "template": only ModelID is required. Existing custom entries are
// archived and the staff is bound to the template.
// Mode "custom":   RotationLength + Entries describe a per-staff pattern.
//
//	Entries that exceed the rotation are rejected.
//	SaveAsTemplateName is optional; when set we additionally
//	create a tenant template from the same payload and bind it.
type scheduleUpdateRequest struct {
	Mode               string                 `json:"mode"`
	ModelID            *int64                 `json:"model_id,omitempty"`
	RotationLength     int                    `json:"rotation_length,omitempty"`
	RotationAnchorDate string                 `json:"rotation_anchor_date,omitempty"`
	Entries            []ScheduleEntryRequest `json:"entries"`
	SaveAsTemplateName string                 `json:"save_as_template,omitempty"`
}

// toServiceInput maps the api request DTO to the service-layer input struct,
// keeping api types out of services/active.
func (req scheduleUpdateRequest) toServiceInput() activeSvc.ScheduleUpdateInput {
	entries := make([]activeSvc.ScheduleEntry, 0, len(req.Entries))
	for _, e := range req.Entries {
		entries = append(entries, activeSvc.ScheduleEntry{
			WeekIndex:     e.WeekIndex,
			DayOfWeek:     e.DayOfWeek,
			TargetMinutes: e.TargetMinutes,
			StartTime:     e.StartTime,
		})
	}
	return activeSvc.ScheduleUpdateInput{
		Mode:               req.Mode,
		ModelID:            req.ModelID,
		RotationLength:     req.RotationLength,
		RotationAnchorDate: req.RotationAnchorDate,
		Entries:            entries,
		SaveAsTemplateName: req.SaveAsTemplateName,
	}
}
