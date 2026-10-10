package timetracking

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// assignmentResponse is the wire format for one Betreuungsplan block a staff
// member is planned into (#1844). It deliberately carries no student data
// (GDPR: no child names in time-tracking lists).
type assignmentResponse struct {
	InstanceID      int64   `json:"instance_id"`
	Title           string  `json:"title"`
	GroupName       *string `json:"group_name,omitempty"`
	RoomName        string  `json:"room_name,omitempty"`
	Date            string  `json:"date"`
	StartTime       string  `json:"start_time"`
	EndTime         string  `json:"end_time"`
	Status          string  `json:"status"`
	IsSpontaneous   bool    `json:"is_spontaneous"`
	Cancelled       bool    `json:"cancelled"`
	IsPrimary       bool    `json:"is_primary"`
	IsSubstitute    bool    `json:"is_substitute"`
	IsAbsent        bool    `json:"is_absent"`
	AbsenceReason   *string `json:"absence_reason,omitempty"`
	CancelReason    *string `json:"cancel_reason,omitempty"`
	UnderstaffedAck bool    `json:"understaffed_ack"`
}

func toAssignmentResponses(assignments []workforce.StaffAssignment) []assignmentResponse {
	out := make([]assignmentResponse, 0, len(assignments))
	for _, a := range assignments {
		out = append(out, assignmentResponse{
			InstanceID:      a.InstanceID,
			Title:           a.Title,
			GroupName:       a.GroupName,
			RoomName:        a.RoomName,
			Date:            a.Date,
			StartTime:       assignmentClock(a.StartTime),
			EndTime:         assignmentClock(a.EndTime),
			Status:          a.Status,
			IsSpontaneous:   a.IsSpontaneous,
			Cancelled:       a.Cancelled,
			IsPrimary:       a.IsPrimary,
			IsSubstitute:    a.IsSubstitute,
			IsAbsent:        a.IsAbsent,
			AbsenceReason:   a.AbsenceReason,
			CancelReason:    a.CancelReason,
			UnderstaffedAck: a.UnderstaffedAck,
		})
	}
	return out
}

// assignmentClock renders a ClockLayout wall clock in the minute precision
// the assignment wire format has always used. A value the contract could not
// have produced passes through unchanged instead of being blanked.
func assignmentClock(value string) string {
	parsed, err := time.Parse(workforce.ClockLayout, value)
	if err != nil {
		return value
	}
	return parsed.Format("15:04")
}
