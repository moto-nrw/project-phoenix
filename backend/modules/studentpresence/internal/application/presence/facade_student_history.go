package presence

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewStudentHistory composes owner attendance reads with visit history,
// data-access logging and planned slot attendance. slots may be nil (no
// timetable wired); slot reads then answer empty.
func NewStudentHistory(attendance AttendanceHistoryReader, rooms HistoryRoomReader, accessLog DataAccessAudit, slots HistorySlotReader) studentpresence.StudentHistory {
	if slots == nil {
		slots = noHistorySlots{}
	}
	return studentHistory{
		StudentHistoryService: NewStudentHistoryService(attendance, rooms),
		HistorySlotReader:     slots,
		accessLog:             accessLog,
	}
}

// studentHistory publishes the history reads. Attendance and visit history
// run through the application service; the slot reads are the timetable
// port under the capability's names and the access log is the audit port.
type studentHistory struct {
	StudentHistoryService
	HistorySlotReader
	accessLog DataAccessAudit
}

func (h studentHistory) RecordDataAccess(ctx context.Context, entry *studentpresence.DataAccessEvent) error {
	return h.accessLog.Create(ctx, dataAccessEvent(entry))
}

// dataAccessEvent maps the public evidence onto the audit record; a scoped
// read keeps its group ids (and date) as the log's metadata.
func dataAccessEvent(entry *studentpresence.DataAccessEvent) *DataAccessEvent {
	if entry == nil {
		return nil
	}
	event := &DataAccessEvent{
		ActorAccountID: entry.ActorAccountID, ActorRole: entry.ActorRole, ResourceType: entry.ResourceType,
		StudentID: entry.StudentID, RangeStart: entry.RangeStart, RangeEnd: entry.RangeEnd, AccessedAt: entry.AccessedAt,
	}
	if scope := entry.Scope; scope != nil {
		event.Metadata = map[string]interface{}{"group_ids": scope.GroupIDs}
		if scope.Date != "" {
			event.Metadata["date"] = scope.Date
		}
	}
	return event
}

// noHistorySlots stands in for a missing timetable: no slot history and no
// planned care.
type noHistorySlots struct{}

func (noHistorySlots) GetSlotAttendanceByStudentAndDateRange(context.Context, int64, calendar.Date, calendar.Date) ([]*studentpresence.HistorySlot, error) {
	return []*studentpresence.HistorySlot{}, nil
}

func (noHistorySlots) HasPlannedSlotsInRange(context.Context, calendar.Date, calendar.Date) (bool, error) {
	return false, nil
}
