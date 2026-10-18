package studentpresencehttp

import (
	context "context"

	studentpresence "github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// StudentPresence is the consumer-owned presence port of the students inbound
// (#3352). It names exactly what the student list, day planning, visit and
// school check-in handlers read and write on the Student Presence capability:
// the shared location snapshot (embedded, which also brings the presence mode
// and the bulk attendance read), today's attendance status, the current and
// past room visits, the present/in-transit/in-room list filters, the session a
// visit belongs to, and the explicit web check-in, check-out and batch
// commands. The composition root binds the public capability to it; handlers
// never see kiosk sessions, student moves or maintenance.
type StudentPresence interface {
	studentpresence.StudentLocationReader
	GetStudentAttendanceStatus(ctx context.Context, studentID int64) (*studentpresence.DailyAttendanceStatus, error)
	GetStudentCurrentVisit(ctx context.Context, studentID int64) (*studentpresence.Visit, error)
	FindVisitsByStudentID(ctx context.Context, studentID int64) ([]studentpresence.Visit, error)
	ListStudentsPresentToday(ctx context.Context) ([]int64, error)
	ListStudentsInTransit(ctx context.Context) ([]int64, error)
	ListStudentsPresentInRoom(ctx context.Context, roomID int64) ([]int64, error)
	GetActiveGroup(ctx context.Context, id int64) (*studentpresence.SessionDetail, error)
	EndVisit(ctx context.Context, id int64) error
	CheckInStudent(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (*studentpresence.AttendanceResult, error)
	CheckOutStudentWithNote(ctx context.Context, studentID, staffID int64, note string, skipAuthCheck bool) (*studentpresence.AttendanceResult, error)
	ProcessSchoolCheckinBatch(ctx context.Context, studentIDs []int64, staffID int64, action string) (*studentpresence.SchoolCheckinBatchResult, error)
}
