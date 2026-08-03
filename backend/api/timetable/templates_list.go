// Package timetable — GET /api/timetable/templates handler.
//
// Read-only list of recurring timetable templates for the planner's
// "Vorlagen" view. This deliberately does not expose edit/delete semantics;
// CRUD follows in the next work package.
package timetable

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/activities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type templateScheduleResponse struct {
	ID               int64  `json:"id"`
	Weekday          int    `json:"weekday"`
	StartTime        string `json:"start_time"`
	EndTime          string `json:"end_time"`
	WeekPattern      int    `json:"week_pattern"`
	CalendarPeriodID *int64 `json:"calendar_period_id,omitempty"`
	// ValidFrom is the inclusive recurrence start (YYYY-MM-DD) set by a
	// template split; empty = period start.
	ValidFrom string `json:"valid_from,omitempty"`
	// ValidUntil is the exclusive recurrence end (YYYY-MM-DD) set by a
	// template split; empty = open-ended.
	ValidUntil string `json:"valid_until,omitempty"`
}

func (rs *Resource) loadTemplates(
	ctx context.Context,
	templateID, calendarPeriodID *int64,
) ([]templateResponse, error) {
	childrenPerStaffRatio := rs.childrenPerStaffRatio(ctx)
	var (
		rows []activities.TemplateListRow
		err  error
	)
	if templateID != nil && calendarPeriodID != nil {
		rows, err = rs.TimetableData.ListTemplateRowsForTemplatePeriod(
			ctx,
			*templateID,
			*calendarPeriodID,
			childrenPerStaffRatio,
		)
	} else {
		rows, err = rs.TimetableData.ListTemplateRows(ctx, templateID, childrenPerStaffRatio)
	}
	if err != nil {
		return nil, err
	}
	weekdayRoster, err := rs.TimetableData.ListTemplateWeekdayRoster(ctx, templateID, calendarPeriodID)
	if err != nil {
		return nil, err
	}
	return mapTemplateRows(rows, childrenPerStaffRatio, weekdayRoster), nil
}

func mapTemplateRows(
	rows []templateRow,
	childrenPerStaffRatio int,
	weekdayRoster []activities.TemplateWeekdayRosterRow,
) []templateResponse {
	assignmentsByTemplate := buildTemplateWeekdayAssignments(weekdayRoster)
	protectedStudentsByTemplate := buildTemplateProtectedStudentAssignments(weekdayRoster)
	templates := make([]templateResponse, 0)
	byID := make(map[int64]int)
	for _, row := range rows {
		idx, ok := byID[row.TemplateID]
		if !ok {
			templates = append(templates, templateResponseFromRow(row, childrenPerStaffRatio))
			idx = len(templates) - 1
			byID[row.TemplateID] = idx
			if assignments, found := assignmentsByTemplate[row.TemplateID]; found {
				templates[idx].WeekdayAssignments = assignments
			}
			if assignments, found := protectedStudentsByTemplate[row.TemplateID]; found {
				templates[idx].ProtectedStudentAssignments = assignments
			}
		}
		templates[idx].Schedules = append(templates[idx].Schedules, templateScheduleResponseFromRow(row))
	}
	return templates
}

func templateResponseFromRow(row templateRow, childrenPerStaffRatio int) templateResponse {
	sourceGradeLevels, err := row.ParseSourceGradeLevels()
	if err != nil {
		// Corrupt jsonb cannot happen via the write path (validated slice);
		// degrade to "no filter" instead of failing the whole list read.
		sourceGradeLevels = nil
	}
	return templateResponse{
		ID:                          row.TemplateID,
		Name:                        row.Name,
		Type:                        row.Type,
		CategoryID:                  row.CategoryID,
		CategoryName:                row.CategoryName,
		RoomID:                      nullableTemplateInt64(row.RoomID.Valid, row.RoomID.Int64),
		RoomName:                    row.RoomName.String,
		EducationGroupID:            educationGroupIDFromRow(row),
		EducationGroupName:          row.EducationGroupName.String,
		IsOpen:                      row.IsOpen,
		MaxParticipants:             row.MaxParticipants,
		CalendarPeriodID:            nullableTemplateInt64(row.TemplateCalendarPeriodID.Valid, row.TemplateCalendarPeriodID.Int64),
		TargetGroupType:             row.TargetGroupType,
		TargetGradeLevel:            nullableTemplateInt16(row.TargetGradeLevel.Valid, row.TargetGradeLevel.Int16),
		TargetSchoolClass:           nullableTemplateString(row.TargetSchoolClass.Valid, row.TargetSchoolClass.String),
		SourceCareOfferingID:        nullableTemplateInt64(row.SourceCareOfferingID.Valid, row.SourceCareOfferingID.Int64),
		SourceGradeLevels:           sourceGradeLevels,
		ListKind:                    nullableTemplateString(row.ListKind.Valid, row.ListKind.String),
		Notes:                       nullableTemplateString(row.Notes.Valid, row.Notes.String),
		ShiftTypeName:               row.ShiftTypeName,
		ShiftTypeColor:              row.ShiftTypeColor,
		EnrollmentCount:             row.EnrollmentCount,
		SupervisorCount:             row.SupervisorCount,
		RequiredStaffCount:          templateRequiredStaffCount(row, childrenPerStaffRatio),
		AssignedStaffCount:          row.CapacitySupervisorCount,
		RequiredStaffOverride:       templateRequiredStaffOverride(row.RequiredStaff),
		StudentIDs:                  row.StudentIDs,
		StaffIDs:                    row.StaffIDs,
		PrimaryStaffID:              nullableTemplateInt64(row.PrimaryStaffID.Valid, row.PrimaryStaffID.Int64),
		Schedules:                   []templateScheduleResponse{},
		WeekdayAssignments:          []templateWeekdayAssignmentResponse{},
		ProtectedStudentAssignments: []templateProtectedStudentAssignmentResponse{},
	}
}

// templateRequiredStaffCount computes the displayed staffing requirement.
// The manual override (#1839) wins over the Betreuungsschlüssel-derived
// value, but only when a real occurrence was selected for capacity scoring:
// a template with no occurrence in the period (every date cancelled,
// AB-week-filtered, or unmaterializable) must not report a false 0/N
// understaffing indicator for a block that never runs.
func templateRequiredStaffCount(row templateRow, childrenPerStaffRatio int) int {
	override := templateRequiredStaffOverride(row.RequiredStaff)
	if !row.CapacityOccurrenceFound {
		override = nil
	}
	return scheduleSvc.EffectiveRequiredStaff(override, row.CapacityEnrollmentCount, childrenPerStaffRatio)
}

// templateRequiredStaffOverride converts the nullable required_staff column
// into the *int override EffectiveRequiredStaff expects (NULL -> nil = derive).
func templateRequiredStaffOverride(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func templateScheduleResponseFromRow(row templateRow) templateScheduleResponse {
	return templateScheduleResponse{
		ID:               row.ScheduleID,
		Weekday:          row.Weekday,
		StartTime:        row.StartTime.String,
		EndTime:          row.EndTime.String,
		WeekPattern:      row.WeekPattern,
		CalendarPeriodID: nullableTemplateInt64(row.CalendarPeriodID.Valid, row.CalendarPeriodID.Int64),
		ValidFrom:        row.ScheduleValidFrom.String,
		ValidUntil:       row.ScheduleValidUntil.String,
	}
}

func nullableTemplateInt64(valid bool, value int64) *int64 {
	if !valid {
		return nil
	}
	return &value
}

func nullableTemplateInt16(valid bool, value int16) *int16 {
	if !valid {
		return nil
	}
	return &value
}

func nullableTemplateString(valid bool, value string) *string {
	if !valid {
		return nil
	}
	return &value
}

func educationGroupIDFromRow(row templateRow) *int64 {
	if !row.EducationGroupID.Valid {
		return nil
	}
	id := row.EducationGroupID.Int64
	return &id
}

type templateResponse struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Type               string `json:"type"`
	CategoryID         int64  `json:"category_id"`
	CategoryName       string `json:"category_name"`
	RoomID             *int64 `json:"room_id,omitempty"`
	RoomName           string `json:"room_name,omitempty"`
	EducationGroupID   *int64 `json:"education_group_id,omitempty"`
	EducationGroupName string `json:"education_group_name,omitempty"`
	IsOpen             bool   `json:"is_open"`
	MaxParticipants    int    `json:"max_participants"`
	// CalendarPeriodID is the template's OWN period pin (distinct from each
	// schedule's own calendar_period_id in templateScheduleResponse).
	CalendarPeriodID *int64 `json:"calendar_period_id,omitempty"`
	// Zielgruppe (target-group) fields — see activities.Group's
	// TargetGroupType* constants ("jahrgang" | "klasse" | "gruppe" |
	// "angebot" | "none").
	TargetGroupType   string  `json:"target_group_type"`
	TargetGradeLevel  *int16  `json:"target_grade_level,omitempty"`
	TargetSchoolClass *string `json:"target_school_class,omitempty"`
	// Offering-source rule (#2137): set only on "angebot" templates whose
	// roster derives from a Betreuungsangebot. Lets the editor prefill the
	// Angebots- and Jahrgangsauswahl on edit.
	SourceCareOfferingID *int64 `json:"source_care_offering_id,omitempty"`
	SourceGradeLevels    []int  `json:"source_grade_levels,omitempty"`
	// ListKind classifies the template for printable daily lists (#1565);
	// nil when the template has no list kind.
	ListKind *string `json:"list_kind,omitempty"`
	// Notes is the template's durable Wochennotiz (#1837 follow-up), nil when
	// no series note is set. Lets the planner prefill the field on a series edit.
	Notes *string `json:"notes,omitempty"`
	// ShiftTypeName/ShiftTypeColor reflect the category's optional
	// Kategorie↔Schichtart mapping (#1836/#1837 follow-up); empty when unmapped.
	ShiftTypeName   string `json:"shift_type_name,omitempty"`
	ShiftTypeColor  string `json:"shift_type_color,omitempty"`
	EnrollmentCount int    `json:"enrollment_count"`
	SupervisorCount int    `json:"supervisor_count"`
	// RequiredStaffCount/AssignedStaffCount drive the Betreuungsplan capacity
	// indicator (issue #1838) — see services/schedule/capacity_service.go.
	RequiredStaffCount int `json:"required_staff_count"`
	AssignedStaffCount int `json:"assigned_staff_count"`
	// RequiredStaffOverride is the raw manual override (#1839), nil when the
	// template derives its requirement from the Betreuungsschlüssel.
	RequiredStaffOverride *int                       `json:"required_staff_override,omitempty"`
	StudentIDs            []int64                    `json:"student_ids"`
	StaffIDs              []int64                    `json:"staff_ids"`
	PrimaryStaffID        *int64                     `json:"primary_staff_id,omitempty"`
	Schedules             []templateScheduleResponse `json:"schedules"`
	// WeekdayAssignments lists the per-weekday roster deviations (#2129).
	// Empty means the series uses one roster on every weekday; when it is
	// populated it covers every weekday the series runs, so a client can
	// render each day's staff and children without further merging.
	WeekdayAssignments []templateWeekdayAssignmentResponse `json:"weekday_assignments"`
	// ProtectedStudentAssignments is read-only provenance metadata. It keeps
	// enrollment-owned selected_weekdays intact when the editor switches from a
	// shared roster to explicit weekday lists.
	ProtectedStudentAssignments []templateProtectedStudentAssignmentResponse `json:"protected_student_assignments"`
}

type listTemplatesResponse struct {
	Templates []templateResponse `json:"templates"`
}

// templateRow aliases the repository read model (issue #584: the
// aggregation queries moved into the activities GroupRepository).
type templateRow = activities.TemplateListRow

func (rs *Resource) listTemplates(w http.ResponseWriter, r *http.Request) {
	if rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("no tenant in context")))
		return
	}

	periodID, ok := templatePeriodIDFromRequest(w, r)
	if !ok {
		return
	}

	// WP-B6: the people subqueries (enrollment_count / supervisor_count) are
	// deliberately NOT filtered by calendar_period_id. The roster of a
	// template that is shown is always shown — rosters are period-scoped at
	// write time (see templates_people.go), so a card visible under an
	// overlapping period must still display its real headcount instead of
	// "0 Kinder". Only the schedule join below stays period-filtered, which
	// decides WHETHER the card appears at all.
	childrenPerStaffRatio := rs.childrenPerStaffRatio(r.Context())
	rows, err := rs.TimetableData.ListTemplateRowsForPeriod(r.Context(), periodID, childrenPerStaffRatio)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("list templates failed", err))
		return
	}
	var weekdayRoster []activities.TemplateWeekdayRosterRow
	if periodID != nil {
		weekdayRoster, err = rs.TimetableData.ListTemplateWeekdayRoster(r.Context(), nil, periodID)
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("list templates failed", err))
			return
		}
	}

	templates := mapTemplateRows(rows, childrenPerStaffRatio, weekdayRoster)
	if periodID == nil {
		// Period-free reads support the enrollment catalog, which only needs
		// template metadata. A nil slice serializes as null and makes clear that
		// weekday rosters were not loaded; [] would falsely claim that the
		// template uses only its shared roster and could be saved lossily.
		for i := range templates {
			templates[i].WeekdayAssignments = nil
			templates[i].ProtectedStudentAssignments = nil
		}
	}
	common.Respond(w, r, http.StatusOK, listTemplatesResponse{
		Templates: templates,
	}, "Templates retrieved")
}

func templatePeriodIDFromRequest(w http.ResponseWriter, r *http.Request) (*int64, bool) {
	raw := r.URL.Query().Get("period_id")
	if raw == "" {
		return nil, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("period_id must be a positive integer")))
		return nil, false
	}
	return &id, true
}
