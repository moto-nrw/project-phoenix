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
// newest first. The arrival is that row's check-in, unless earlier stays ended
// with a checkout to class: a return from class is no new arrival (#5656). A
// return after going home still is one.
func appendSchoolStatuses(result []studentpresence.SchoolStatus, ids []int64, date string, rows []*ports.Attendance) []studentpresence.SchoolStatus {
	latest, arrival := latestAndArrivalRows(rows)
	for _, id := range ids {
		status := studentpresence.SchoolStatus{StudentID: id, Date: date, Status: "not_checked_in"}
		if row := latest[id]; row != nil {
			first := arrival[id]
			status.ArrivalTime = &first.CheckInTime
			status.CheckedInBy, status.CheckedOutBy, status.CheckedOutDeviceID = first.CheckedInBy, row.CheckedOutBy, row.CheckedOutDeviceID
			status.CheckInTime, status.CheckOutTime, status.YardSince = &row.CheckInTime, row.CheckOutTime, row.YardSince
			status.CheckOutNote, status.CheckOutReason = row.CheckOutNote, row.CheckOutReason
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

// latestAndArrivalRows reads rows that arrive per student, newest first. The
// arrival walks back from the newest row for as long as the stay before it
// ended with a checkout to class.
func latestAndArrivalRows(rows []*ports.Attendance) (latest, arrival map[int64]*ports.Attendance) {
	latest = make(map[int64]*ports.Attendance, len(rows))
	arrival = make(map[int64]*ports.Attendance, len(rows))
	settled := make(map[int64]bool, len(rows))
	for _, row := range rows {
		id := row.StudentID
		switch {
		case latest[id] == nil:
			latest[id], arrival[id] = row, row
		case settled[id]:
		case row.CheckOutReason != nil && *row.CheckOutReason == studentpresence.CheckoutReasonClass:
			arrival[id] = row
		default:
			settled[id] = true
		}
	}
	return latest, arrival
}
