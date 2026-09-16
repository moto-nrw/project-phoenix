package timetracking

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// Employment types a staff row may carry; mirrors the legacy model constants,
// the same way modules/schoolmembership names them for its own surface.
//
// The value is simultaneously the stored column, the wire field the frontend
// reads, and the key the export labels are looked up by, so translating it to
// an internal enum at the boundary would only convert it straight back. The
// owner's Validate accepts exactly these three and nothing links the two
// copies at compile time, so widening the set means changing both.
const (
	EmploymentTypeFullTime = "full_time"
	EmploymentTypePartTime = "part_time"
	EmploymentTypeMinijob  = "minijob"
)

// StaffScheduleBinding is the part of a staff row the schedule commands
// rewrite: the assigned work time model and the rotation anchor.
type StaffScheduleBinding struct {
	ID                 int64
	WorkTimeModelID    *int64
	RotationAnchorDate *timezone.Date
}
