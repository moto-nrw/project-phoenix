// Package timetable — GET /api/timetable/templates handler.
//
// Read-only list of recurring timetable templates for the planner's
// "Vorlagen" view. This deliberately does not expose edit/delete semantics;
// CRUD follows in the next work package.
package timetable

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type templateScheduleResponse struct {
	ID               int64  `json:"id"`
	Weekday          int    `json:"weekday"`
	StartTime        string `json:"start_time"`
	EndTime          string `json:"end_time"`
	WeekPattern      int    `json:"week_pattern"`
	CalendarPeriodID *int64 `json:"calendar_period_id,omitempty"`
}

type templateResponse struct {
	ID              int64                      `json:"id"`
	Name            string                     `json:"name"`
	Type            string                     `json:"type"`
	CategoryID      int64                      `json:"category_id"`
	CategoryName    string                     `json:"category_name"`
	RoomID          *int64                     `json:"room_id,omitempty"`
	RoomName        string                     `json:"room_name,omitempty"`
	IsOpen          bool                       `json:"is_open"`
	MaxParticipants int                        `json:"max_participants"`
	EnrollmentCount int                        `json:"enrollment_count"`
	SupervisorCount int                        `json:"supervisor_count"`
	Schedules       []templateScheduleResponse `json:"schedules"`
}

type listTemplatesResponse struct {
	Templates []templateResponse `json:"templates"`
}

type templateRow struct {
	TemplateID       int64          `bun:"template_id"`
	Name             string         `bun:"name"`
	Type             string         `bun:"type"`
	CategoryID       int64          `bun:"category_id"`
	CategoryName     string         `bun:"category_name"`
	RoomID           sql.NullInt64  `bun:"room_id"`
	RoomName         sql.NullString `bun:"room_name"`
	IsOpen           bool           `bun:"is_open"`
	MaxParticipants  int            `bun:"max_participants"`
	EnrollmentCount  int            `bun:"enrollment_count"`
	SupervisorCount  int            `bun:"supervisor_count"`
	ScheduleID       int64          `bun:"schedule_id"`
	Weekday          int            `bun:"weekday"`
	StartTime        sql.NullString `bun:"start_time"`
	EndTime          sql.NullString `bun:"end_time"`
	WeekPattern      int            `bun:"week_pattern"`
	CalendarPeriodID sql.NullInt64  `bun:"calendar_period_id"`
}

func (rs *Resource) listTemplates(w http.ResponseWriter, r *http.Request) {
	if rs.db == nil {
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

	rows := make([]templateRow, 0)
	query := `
		SELECT
			g.id AS template_id,
			g.name,
			g.type,
			g.category_id,
			COALESCE(c.name, '') AS category_name,
			g.planned_room_id AS room_id,
			COALESCE(r.name, '') AS room_name,
			g.is_open,
			g.max_participants,
			COALESCE(enrollments.count, 0) AS enrollment_count,
			COALESCE(supervisors.count, 0) AS supervisor_count,
			s.id AS schedule_id,
			s.weekday,
			COALESCE(TO_CHAR(tf.start_time, 'HH24:MI'), '') AS start_time,
			COALESCE(TO_CHAR(tf.end_time, 'HH24:MI'), '') AS end_time,
			s.week_pattern,
			s.calendar_period_id
		FROM activities.groups AS g
		INNER JOIN activities.schedules AS s
			ON s.activity_group_id = g.id AND s.tenant_id = g.tenant_id
		LEFT JOIN schedule.timeframes AS tf
			ON tf.id = s.timeframe_id AND tf.tenant_id = g.tenant_id
		LEFT JOIN activities.categories AS c
			ON c.id = g.category_id AND c.tenant_id = g.tenant_id
		LEFT JOIN facilities.rooms AS r
			ON r.id = g.planned_room_id AND r.tenant_id = g.tenant_id
		LEFT JOIN (
			SELECT activity_group_id, COUNT(*) AS count
			FROM activities.student_enrollments
			WHERE tenant_id = ?
			GROUP BY activity_group_id
		) AS enrollments ON enrollments.activity_group_id = g.id
		LEFT JOIN (
			SELECT group_id, COUNT(*) AS count
			FROM activities.supervisors
			WHERE tenant_id = ?
			GROUP BY group_id
		) AS supervisors ON supervisors.group_id = g.id
		WHERE g.tenant_id = ?
		  AND g.is_template = true`

	args := []any{tenantID, tenantID, tenantID}
	if periodID != nil {
		query += ` AND s.calendar_period_id = ?`
		args = append(args, *periodID)
	}
	query += ` ORDER BY g.name ASC, s.weekday ASC, tf.start_time ASC`

	if err := base.GetDB(r.Context(), rs.db).NewRaw(query, args...).Scan(r.Context(), &rows); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("list templates failed", err))
		return
	}

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
			templates = append(templates, templateResponse{
				ID:              row.TemplateID,
				Name:            row.Name,
				Type:            row.Type,
				CategoryID:      row.CategoryID,
				CategoryName:    row.CategoryName,
				RoomID:          roomID,
				RoomName:        row.RoomName.String,
				IsOpen:          row.IsOpen,
				MaxParticipants: row.MaxParticipants,
				EnrollmentCount: row.EnrollmentCount,
				SupervisorCount: row.SupervisorCount,
				Schedules:       []templateScheduleResponse{},
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
		})
	}

	common.Respond(w, r, http.StatusOK, listTemplatesResponse{Templates: templates}, "Templates retrieved")
}
