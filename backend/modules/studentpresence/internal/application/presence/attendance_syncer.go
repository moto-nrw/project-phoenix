package presence

import "github.com/moto-nrw/project-phoenix/modules/studentpresence"

// attendanceDetail carries the three WP-B10 fields of a snapshot into the
// realtime event. A nil snapshot yields nil, so the event goes out with
// attendance_* omitted. Status is always set when the snapshot is non-nil
// (it's a required column); Substatus and Note may be nil.
func attendanceDetail(snapshot *AttendanceSnapshot) *studentpresence.LiveAttendanceDetail {
	if snapshot == nil {
		return nil
	}
	return &studentpresence.LiveAttendanceDetail{Status: snapshot.Status, Substatus: snapshot.Substatus, Note: snapshot.Note}
}
