package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ListSchoolStatuses returns one status per requested student. Checkout wins
// over yard presence; a student without a stay has not_checked_in status.
func (s *Service) ListSchoolStatuses(ctx context.Context, ids []int64, date string) (result []studentpresence.SchoolStatus, err error) {
	if _, err := calendar.ParseDate(date); err != nil {
		return nil, err
	}
	err = s.runWrite(ctx, "list_school_statuses", func(ctx context.Context) (ports.Stats, error) {
		if date == "" || !validIDs(ids) {
			return ports.Stats{}, errors.New("student presence: invalid school status query")
		}
		result = make([]studentpresence.SchoolStatus, 0, len(ids))
		if len(ids) == 0 {
			return ports.Stats{}, nil
		}
		rows, stats, err := s.store.ListAttendance(ctx, studentpresence.AttendanceFilter{
			StudentIDs: ids, FromDate: date, UntilDate: date, StudentOrder: true, NewestFirst: true,
		})
		if err != nil {
			return stats, err
		}
		result = appendSchoolStatuses(result, ids, date, rows)
		return stats, nil
	})
	return result, err
}

// appendSchoolStatuses appends one status per requested student, derived from
// the student's newest attendance row of the day; rows arrive per student,
// newest first.
func appendSchoolStatuses(result []studentpresence.SchoolStatus, ids []int64, date string, rows []*ports.Attendance) []studentpresence.SchoolStatus {
	latest := make(map[int64]*ports.Attendance, len(rows))
	for _, row := range rows {
		if _, found := latest[row.StudentID]; !found {
			latest[row.StudentID] = row
		}
	}
	for _, id := range ids {
		status := studentpresence.SchoolStatus{StudentID: id, Date: date, Status: "not_checked_in"}
		if row := latest[id]; row != nil {
			status.CheckedInBy, status.CheckedOutBy, status.CheckedOutDeviceID = row.CheckedInBy, row.CheckedOutBy, row.CheckedOutDeviceID
			status.CheckInTime, status.CheckOutTime, status.YardSince = &row.CheckInTime, row.CheckOutTime, row.YardSince
			status.CheckOutNote = row.CheckOutNote
			switch {
			case row.CheckOutTime != nil:
				status.Status = "checked_out"
			case row.YardSince != nil:
				status.Status = "on_yard"
			default:
				status.Status = "checked_in"
			}
		}
		result = append(result, status)
	}
	return result
}
