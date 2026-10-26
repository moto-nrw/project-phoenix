package owners

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
)

// attendanceNameStaff reads staff members of the school in context with
// their persons, retired members included.
type attendanceNameStaff interface {
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.Staff, error)
}

type attendanceStaffNames struct {
	staff attendanceNameStaff
}

// NewAttendanceStaffNames names the staff members behind attendance records
// (#4761). The membership read is tenant-scoped, so only staff of the
// child's school resolve.
func NewAttendanceStaffNames(staff attendanceNameStaff) presenceservice.AttendanceStaffNames {
	return attendanceStaffNames{staff: staff}
}

// StaffNames keeps retired staff named while their records are kept. The
// automatic deletion of staff anonymizes and soft-deletes the person of an
// erased tombstone, so the person read finds none and the record names a
// former staff member instead.
func (q attendanceStaffNames) StaffNames(ctx context.Context, ids []int64) (map[int64]string, error) {
	names := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return names, nil
	}
	rows, err := q.staff.FindWithPersonByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for id, row := range rows {
		if row == nil || row.Person == nil {
			continue
		}
		if name := row.Person.GetFullName(); name != "" {
			names[id] = name
		}
	}
	return names, nil
}
