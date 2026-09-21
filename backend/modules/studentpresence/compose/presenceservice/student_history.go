package presenceservice

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
)

// NewStudentHistory composes owner attendance reads with visit history,
// data-access logging and planned slot attendance. slots may be nil (no
// timetable wired); slot reads then answer empty.
func NewStudentHistory(attendance AttendanceHistoryReader, rooms HistoryRoomReader, accessLog DataAccessAudit, slots HistorySlotReader) studentpresence.StudentHistory {
	if slots == nil {
		slots = noHistorySlots{}
	}
	return studentHistory{
		StudentHistoryService: presence.NewStudentHistoryService(attendance, rooms),
		accessLog:             accessLog,
		slots:                 slots,
	}
}

// studentHistory publishes the history reads. Attendance and visit history
// run through the application service; the slot reads and the access log are
// the timetable and audit ports under the capability's names.
type studentHistory struct {
	presence.StudentHistoryService
	accessLog DataAccessAudit
	slots     HistorySlotReader
}

func (h studentHistory) GetSlotAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*studentpresence.HistorySlot, error) {
	return h.slots.Slots(ctx, studentID, startDate, endDate)
}

func (h studentHistory) HasPlannedSlotsInRange(ctx context.Context, startDate, endDate timezone.Date) (bool, error) {
	return h.slots.HasPlannedSlots(ctx, startDate, endDate)
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

func (noHistorySlots) Slots(context.Context, int64, timezone.Date, timezone.Date) ([]*studentpresence.HistorySlot, error) {
	return []*studentpresence.HistorySlot{}, nil
}

func (noHistorySlots) HasPlannedSlots(context.Context, timezone.Date, timezone.Date) (bool, error) {
	return false, nil
}
