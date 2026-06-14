// Package timetable — GET /api/timetable/templates handler.
//
// Read-only list of recurring timetable templates for the planner's
// "Vorlagen" view. This deliberately does not expose edit/delete semantics;
// CRUD follows in the next work package.
package timetable

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type templateScheduleResponse struct {
	ID               int64  `json:"id"`
	Weekday          int    `json:"weekday"`
	StartTime        string `json:"start_time"`
	EndTime          string `json:"end_time"`
	WeekPattern      int    `json:"week_pattern"`
	CalendarPeriodID *int64 `json:"calendar_period_id,omitempty"`
	// ValidUntil is the exclusive recurrence end (YYYY-MM-DD) set by a
	// template split; empty = open-ended.
	ValidUntil string `json:"valid_until,omitempty"`
}

func (rs *Resource) loadTemplates(ctx context.Context, templateID *int64) ([]templateResponse, error) {
	rows, err := rs.timetableData.ListTemplateRows(ctx, templateID)
	if err != nil {
		return nil, err
	}
	return mapTemplateRows(rows), nil
}

func (rs *Resource) templateExists(ctx context.Context, templateID int64) (bool, error) {
	templates, err := rs.loadTemplates(ctx, &templateID)
	if err != nil {
		return false, err
	}
	return len(templates) > 0, nil
}

func mapTemplateRows(rows []templateRow) []templateResponse {
	templates := make([]templateResponse, 0)
	byID := make(map[int64]int)
	for _, row := range rows {
		idx, ok := byID[row.TemplateID]
		if !ok {
			var roomID *int64
			if row.RoomID.Valid {
				id := row.RoomID.Int64
				roomID = &id
			}
			var primaryStaffID *int64
			if row.PrimaryStaffID.Valid {
				id := row.PrimaryStaffID.Int64
				primaryStaffID = &id
			}
			templates = append(templates, templateResponse{
				ID:                 row.TemplateID,
				Name:               row.Name,
				Type:               row.Type,
				CategoryID:         row.CategoryID,
				CategoryName:       row.CategoryName,
				RoomID:             roomID,
				RoomName:           row.RoomName.String,
				EducationGroupID:   educationGroupIDFromRow(row),
				EducationGroupName: row.EducationGroupName.String,
				IsOpen:             row.IsOpen,
				MaxParticipants:    row.MaxParticipants,
				EnrollmentCount:    row.EnrollmentCount,
				SupervisorCount:    row.SupervisorCount,
				StudentIDs:         row.StudentIDs,
				StaffIDs:           row.StaffIDs,
				PrimaryStaffID:     primaryStaffID,
				Schedules:          []templateScheduleResponse{},
			})
			idx = len(templates) - 1
			byID[row.TemplateID] = idx
		}

		var calendarPeriodID *int64
		if row.CalendarPeriodID.Valid {
			id := row.CalendarPeriodID.Int64
			calendarPeriodID = &id
		}
		templates[idx].Schedules = append(templates[idx].Schedules, templateScheduleResponse{
			ID:               row.ScheduleID,
			Weekday:          row.Weekday,
			StartTime:        row.StartTime.String,
			EndTime:          row.EndTime.String,
			WeekPattern:      row.WeekPattern,
			CalendarPeriodID: calendarPeriodID,
			ValidUntil:       row.ScheduleValidUntil.String,
		})
	}
	return templates
}

func educationGroupIDFromRow(row templateRow) *int64 {
	if !row.EducationGroupID.Valid {
		return nil
	}
	id := row.EducationGroupID.Int64
	return &id
}

type templateResponse struct {
	ID                 int64                      `json:"id"`
	Name               string                     `json:"name"`
	Type               string                     `json:"type"`
	CategoryID         int64                      `json:"category_id"`
	CategoryName       string                     `json:"category_name"`
	RoomID             *int64                     `json:"room_id,omitempty"`
	RoomName           string                     `json:"room_name,omitempty"`
	EducationGroupID   *int64                     `json:"education_group_id,omitempty"`
	EducationGroupName string                     `json:"education_group_name,omitempty"`
	IsOpen             bool                       `json:"is_open"`
	MaxParticipants    int                        `json:"max_participants"`
	EnrollmentCount    int                        `json:"enrollment_count"`
	SupervisorCount    int                        `json:"supervisor_count"`
	StudentIDs         []int64                    `json:"student_ids"`
	StaffIDs           []int64                    `json:"staff_ids"`
	PrimaryStaffID     *int64                     `json:"primary_staff_id,omitempty"`
	Schedules          []templateScheduleResponse `json:"schedules"`
}

type listTemplatesResponse struct {
	Templates []templateResponse `json:"templates"`
}

// templateRow aliases the repository read model (issue #584: the
// aggregation queries moved into the activities GroupRepository).
type templateRow = activities.TemplateListRow

func (rs *Resource) listTemplates(w http.ResponseWriter, r *http.Request) {
	if rs.timetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("no tenant in context")))
		return
	}

	var periodID *int64
	if raw := r.URL.Query().Get("period_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("period_id must be a positive integer")))
			return
		}
		periodID = &id
	}

	// WP-B6: the people subqueries (enrollment_count / supervisor_count) are
	// deliberately NOT filtered by calendar_period_id. The roster of a
	// template that is shown is always shown — rosters are period-scoped at
	// write time (see templates_people.go), so a card visible under an
	// overlapping period must still display its real headcount instead of
	// "0 Kinder". Only the schedule join below stays period-filtered, which
	// decides WHETHER the card appears at all.
	rows, err := rs.timetableData.ListTemplateRowsForPeriod(r.Context(), periodID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("list templates failed", err))
		return
	}

	common.Respond(w, r, http.StatusOK, listTemplatesResponse{Templates: mapTemplateRows(rows)}, "Templates retrieved")
}
