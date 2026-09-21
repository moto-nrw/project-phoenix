package presence

import (
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
)

// attendanceDetail carries the three WP-B10 fields of a snapshot into the
// realtime event. A nil snapshot yields nil, so the event goes out with
// attendance_* omitted. Status is always set when the snapshot is non-nil
// (it's a required column); Substatus and Note may be nil.
func attendanceDetail(snapshot *AttendanceSnapshot) *realtimeevents.AttendanceDetail {
	if snapshot == nil {
		return nil
	}
	return &realtimeevents.AttendanceDetail{Status: snapshot.Status, Substatus: snapshot.Substatus, Note: snapshot.Note}
}
