package compose

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// ScheduledInstanceOf maps a retained instance row, whose status carries the
// Student Presence session state, onto the owner's scheduled-instance view.
func ScheduledInstanceOf(row *scheduleModels.ActivityInstance) timetable.ScheduledInstance {
	return timetable.ScheduledInstance{
		ID:                    row.ID,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
		TenantID:              row.TenantID,
		Date:                  timezone.Date(row.Date),
		ActivityGroupID:       row.ActivityGroupID,
		CalendarPeriodID:      row.CalendarPeriodID,
		Title:                 row.Title,
		Description:           row.Description,
		StartTime:             row.StartTime,
		EndTime:               row.EndTime,
		RoomID:                row.RoomID,
		RequiredStaff:         row.RequiredStaff,
		Status:                row.Status,
		ActiveGroupID:         row.ActiveGroupID,
		ListKind:              row.ListKind,
		IsSpontaneous:         row.IsSpontaneous,
		UnderstaffedAck:       row.UnderstaffedAck,
		UnderstaffedNote:      row.UnderstaffedNote,
		CancelReason:          row.CancelReason,
		Notes:                 row.Notes,
		CreatedBy:             row.CreatedBy,
		StartedBy:             row.StartedBy,
		StartedAt:             row.StartedAt,
		CompletedAt:           row.CompletedAt,
		CompletedBy:           row.CompletedBy,
		ReopenUntil:           row.ReopenUntil,
		HasCompletionSnapshot: len(row.CompletionSnapshot) > 0,
	}
}
