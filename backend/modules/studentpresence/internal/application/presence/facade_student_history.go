package presence

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewStudentHistory composes owner attendance reads with visit history,
// data-access logging, planned slot attendance and the names of the staff
// who recorded attendance. slots may be nil (no timetable wired); slot reads
// then answer empty.
func NewStudentHistory(attendance AttendanceHistoryReader, rooms HistoryRoomReader, accessLog DataAccessAudit, slots HistorySlotReader, staff AttendanceStaffNames) studentpresence.StudentHistory {
	if slots == nil {
		slots = noHistorySlots{}
	}
	return studentHistory{
		StudentHistoryService: NewStudentHistoryService(attendance, rooms),
		HistorySlotReader:     slots,
		accessLog:             accessLog,
		staff:                 staff,
	}
}

// studentHistory publishes the history reads. Attendance and visit history
// run through the application service; the slot reads are the timetable
// port under the capability's names and the access log is the audit port.
type studentHistory struct {
	StudentHistoryService
	HistorySlotReader
	accessLog DataAccessAudit
	staff     AttendanceStaffNames
}

// AttendanceRecorders reads the names of every staff member behind rows at
// once. A failed read is an error: the history must not present a known
// staff member as a former one.
func (h studentHistory) AttendanceRecorders(ctx context.Context, rows []*studentpresence.Attendance) (studentpresence.AttendanceRecorders, error) {
	values := make([]studentpresence.Attendance, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			values = append(values, *row)
		}
	}
	ids := studentpresence.AttendanceStaffIDs(values)
	if len(ids) == 0 {
		return studentpresence.NewAttendanceRecorders(nil), nil
	}
	if h.staff == nil {
		return studentpresence.AttendanceRecorders{}, errors.New("student history: attendance staff names are not bound")
	}
	names, err := h.staff.StaffNames(ctx, ids)
	if err != nil {
		return studentpresence.AttendanceRecorders{}, err
	}
	return studentpresence.NewAttendanceRecorders(names), nil
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
