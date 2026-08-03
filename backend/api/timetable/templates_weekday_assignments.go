// Package timetable — per-weekday roster assignments on a recurring template
// (issue #2129).
//
// A Regeltermin that runs Monday to Friday used to carry ONE staff and child
// roster for the whole series. Schools whose daily routine is identical but
// whose responsibilities rotate had to model the same block five times. The
// `weekday_assignments` field on the create/update/split bodies expresses the
// difference directly: the top-level `staff_ids` / `student_ids` /
// `primary_staff_id` stay the shared default, and each entry here overrides
// them for exactly one weekday.
package timetable

import (
	"errors"
	"fmt"

	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// weekdayAssignmentRequest is one weekday's roster deviation. It is a full
// replacement for that weekday, not a delta: omitting a staff member from
// `staff_ids` removes them from that weekday. That keeps the wire shape
// explicit instead of encoding add/remove semantics into flat id arrays.
type weekdayAssignmentRequest struct {
	Weekday        int     `json:"weekday"` // ISO 8601: 1=Mon … 7=Sun
	StaffIDs       []int64 `json:"staff_ids"`
	StudentIDs     []int64 `json:"student_ids"`
	PrimaryStaffID *int64  `json:"primary_staff_id,omitempty"`
}

// validateWeekdayAssignments checks the deviations against the template's own
// weekday list. Rejecting an assignment for an unscheduled weekday is
// deliberate: silently dropping it loses the planner's input, and silently
// adding the weekday would change the recurrence behind their back.
func validateWeekdayAssignments(assignments []weekdayAssignmentRequest, weekdays []int) error {
	if len(assignments) == 0 {
		return nil
	}
	scheduled := make(map[int]struct{}, len(weekdays))
	for _, weekday := range weekdays {
		scheduled[weekday] = struct{}{}
	}
	seen := make(map[int]struct{}, len(assignments))
	for _, assignment := range assignments {
		if !activitiesModel.IsValidWeekday(assignment.Weekday) {
			return fmt.Errorf("invalid weekday %d in weekday_assignments (must be 1=Mon … 7=Sun)", assignment.Weekday)
		}
		if _, ok := scheduled[assignment.Weekday]; !ok {
			return fmt.Errorf("weekday_assignments contains weekday %d, which is not part of weekdays", assignment.Weekday)
		}
		if _, dup := seen[assignment.Weekday]; dup {
			return fmt.Errorf("weekday_assignments lists weekday %d twice", assignment.Weekday)
		}
		seen[assignment.Weekday] = struct{}{}
		if assignment.PrimaryStaffID != nil && !containsID(assignment.StaffIDs, *assignment.PrimaryStaffID) {
			return errors.New("primary_staff_id must be part of the same weekday's staff_ids")
		}
	}
	return nil
}

func containsID(ids []int64, want int64) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// toServiceWeekdayAssignments maps the wire shape onto the service input.
func toServiceWeekdayAssignments(assignments []weekdayAssignmentRequest) []scheduleSvc.WeekdayRosterAssignment {
	if len(assignments) == 0 {
		return nil
	}
	out := make([]scheduleSvc.WeekdayRosterAssignment, 0, len(assignments))
	for _, assignment := range assignments {
		out = append(out, scheduleSvc.WeekdayRosterAssignment{
			Weekday:        assignment.Weekday,
			StudentIDs:     assignment.StudentIDs,
			StaffIDs:       assignment.StaffIDs,
			PrimaryStaffID: assignment.PrimaryStaffID,
		})
	}
	return out
}

// templateWeekdayAssignmentResponse mirrors weekdayAssignmentRequest on the
// read side. A template whose roster is identical on every weekday returns an
// empty list, which is how clients tell "no deviations" from "deviations".
type templateWeekdayAssignmentResponse struct {
	Weekday        int     `json:"weekday"`
	StaffIDs       []int64 `json:"staff_ids"`
	StudentIDs     []int64 `json:"student_ids"`
	PrimaryStaffID *int64  `json:"primary_staff_id,omitempty"`
}

// buildTemplateWeekdayAssignments groups the flat, kind-tagged roster rows the
// repository returns into one entry per weekday, keyed by template id.
func buildTemplateWeekdayAssignments(
	rows []activitiesModel.TemplateWeekdayRosterRow,
) map[int64][]templateWeekdayAssignmentResponse {
	// The repository orders by (template, weekday, kind, is_primary, person),
	// so appending in scan order already yields stable, sorted output.
	byTemplate := make(map[int64][]templateWeekdayAssignmentResponse)
	index := make(map[int64]map[int]int)
	for _, row := range rows {
		perWeekday, ok := index[row.TemplateID]
		if !ok {
			perWeekday = make(map[int]int)
			index[row.TemplateID] = perWeekday
		}
		position, ok := perWeekday[row.Weekday]
		if !ok {
			byTemplate[row.TemplateID] = append(byTemplate[row.TemplateID], templateWeekdayAssignmentResponse{
				Weekday:    row.Weekday,
				StaffIDs:   []int64{},
				StudentIDs: []int64{},
			})
			position = len(byTemplate[row.TemplateID]) - 1
			perWeekday[row.Weekday] = position
		}
		entry := &byTemplate[row.TemplateID][position]
		switch row.Kind {
		case activitiesModel.TemplateWeekdayRosterKindEmpty:
			// The entry itself is the payload: no person belongs to this day.
		case activitiesModel.TemplateWeekdayRosterKindStaff:
			entry.StaffIDs = append(entry.StaffIDs, row.PersonID)
			if row.IsPrimary && entry.PrimaryStaffID == nil {
				staffID := row.PersonID
				entry.PrimaryStaffID = &staffID
			}
		case activitiesModel.TemplateWeekdayRosterKindStudent:
			entry.StudentIDs = append(entry.StudentIDs, row.PersonID)
		}
	}
	return byTemplate
}
